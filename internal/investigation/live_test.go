package investigation

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OpenWAF/internal/api"
	"OpenWAF/internal/assistant"
	"OpenWAF/internal/auth"
	"OpenWAF/internal/database"
	"OpenWAF/internal/domain"
	"OpenWAF/internal/telemetry"
	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

var liveInvestigation = flag.Bool("live-investigation", false, "Run paid provider tests against isolated synthetic traffic")

type liveEvent struct {
	Event domain.InvestigationEvent
	Raw   []byte
	Err   error
}

func TestLiveInvestigation(t *testing.T) {
	if !*liveInvestigation && os.Getenv("OPENWAF_LIVE_AI") != "1" {
		t.Skip("enable -live-investigation to call the configured provider")
	}
	config, err := godotenv.Read(filepath.Join("..", "..", ".env"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal("could not read .env")
	}
	for _, key := range []string{"AI_KEY", "AI_MODEL", "ASSISTANT_BASE_URL"} {
		if _, ok := os.LookupEnv(key); !ok {
			t.Setenv(key, config[key])
		}
	}
	if os.Getenv("AI_KEY") == "" || os.Getenv("AI_MODEL") == "" {
		t.Fatal("AI_KEY and AI_MODEL must be configured")
	}
	db, err := database.Open(filepath.Join(t.TempDir(), "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	ts := telemetry.New(database.NewStore(db))
	ai, err := assistant.New(context.Background(), db, ts)
	if err != nil {
		sqlDB.Close()
		t.Fatal("could not initialize the configured AI provider")
	}
	s, err := New(db, ai, ts)
	if err != nil {
		ai.Close()
		sqlDB.Close()
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{ErrorHandler: api.ErrorHandler})
	authService, err := auth.New(database.NewStore(db))
	if err != nil {
		t.Fatal(err)
	}
	router := api.New(app).Group("/api")
	authHandler := auth.NewHandler(authService, false, nil)
	authHandler.Register(router)
	assistant.NewHandler(ai).Register(router, authHandler.RequireAuth)
	NewHandler(s).Register(router, authHandler.RequireAuth)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + listener.Addr().String()
	serverDone := make(chan error, 1)
	go func() { serverDone <- app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true}) }()
	workerCtx, stopWorker := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	workerStarted := false
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 8 * time.Minute}
	t.Cleanup(func() {
		stopWorker()
		if workerStarted {
			<-workerDone
		}
		ai.Close()
		client.CloseIdleConnections()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		app.ShutdownWithContext(ctx)
		listener.Close()
		<-serverDone
		sqlDB.Close()
	})
	call := func(method, path, body string, want int) []byte {
		t.Helper()
		request, err := http.NewRequest(method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s: status=%d want=%d response=%s", method, path, response.StatusCode, want, data)
		}
		assertPublicPayload(t, data)
		return data
	}
	call("GET", "/api/investigations", "", 401)
	call("POST", "/api/auth/setup", `{"username":"live-investigation","password":"isolated-live-password"}`, 201)
	call("POST", "/api/auth/sign-in", `{"username":"live-investigation","password":"isolated-live-password"}`, 200)
	if _, err := s.aggregate(context.Background()); err != nil {
		t.Fatal(err)
	}
	service := domain.Service{Name: "Mock shop", Hostname: "shop.test", UpstreamURL: "http://127.0.0.1:1", Enabled: true}
	if err := db.Create(&service).Error; err != nil {
		t.Fatal(err)
	}
	seeded := seedLiveTraffic(t, db, service.ID)
	streamCtx, stopStream := context.WithCancel(context.Background())
	defer stopStream()
	events := subscribeLive(t, client, streamCtx, base+"/api/investigations/events?after=0")
	workerStarted = true
	go func() { defer close(workerDone); s.Run(workerCtx) }()
	counts := map[string]int{}
	investigationID := ""
	deadline := time.NewTimer(420 * time.Second)
	defer deadline.Stop()
	complete := false
	for !complete {
		select {
		case item, ok := <-events:
			if !ok {
				t.Fatal("SSE ended before publication")
			}
			if item.Err != nil {
				t.Fatal(item.Err)
			}
			assertPublicPayload(t, item.Raw)
			e := item.Event
			counts[e.Type]++
			if investigationID == "" {
				investigationID = e.InvestigationID
			}
			if e.InvestigationID != investigationID {
				t.Fatalf("unexpected second investigation %s", e.InvestigationID)
			}
			if e.Type == "investigation.failed" {
				t.Fatal("live investigation failed")
			}
			if e.Type == "investigation.completed" {
				complete = true
			}
		case <-deadline.C:
			t.Fatalf("investigation timed out; events=%v", counts)
		}
	}
	for _, kind := range []string{"investigation.created", "investigation.queued", "investigation.started", "activity.started", "activity.completed", "explanation.delta", "investigation.result_published", "investigation.completed"} {
		if counts[kind] == 0 {
			t.Errorf("missing live event %s", kind)
		}
	}
	stopStream()
	path := "/api/investigations/" + investigationID
	var detail investigationDetail
	if err := json.Unmarshal(call("GET", path, "", 200), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.ID != investigationID || detail.Status != "completed" || detail.Result == nil || detail.ResultVersion != 1 || len(detail.Result.Evidence) == 0 {
		t.Fatalf("incomplete published investigation: status=%s version=%d", detail.Status, detail.ResultVersion)
	}
	if detail.Trigger.Detector != "path_scanning" || detail.Trigger.Observed != 40 || detail.Trigger.Threshold != 20 || detail.Trigger.Explanation == "" {
		t.Fatalf("unexpected trigger: %+v", detail.Trigger)
	}
	for _, evidence := range detail.Result.Evidence {
		if !seeded[evidence.Request.ID] || !evidence.Available {
			t.Fatalf("unverified request evidence %d", evidence.Request.ID)
		}
	}
	t.Logf("automatic investigation: severity=%s assessment=%s verified_requests=%d events=%v", detail.Result.Severity, detail.Result.Assessment, len(detail.Result.Evidence), counts)
	var list investigationList
	if err := json.Unmarshal(call("GET", "/api/investigations?serviceId="+fmt.Sprint(service.ID)+"&status=completed&unread=true", "", 200), &list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != investigationID || list.Items[0].Read {
		t.Fatal("unified list or unread filtering failed")
	}
	call("PATCH", path, `{"read":true,"resultVersion":1}`, 200)
	var summary investigationSummary
	if err := json.Unmarshal(call("GET", "/api/investigations/summary", "", 200), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Total != 1 || summary.Unread != 0 || summary.ByStatus["completed"] != 1 {
		t.Fatalf("incorrect summary: %+v", summary)
	}
	var conversation domain.AssistantConversation
	if err := json.Unmarshal(call("POST", path+"/follow-up", "", 201), &conversation); err != nil {
		t.Fatal(err)
	}
	var same domain.AssistantConversation
	if err := json.Unmarshal(call("POST", path+"/follow-up", "", 200), &same); err != nil {
		t.Fatal(err)
	}
	if same.ID != conversation.ID {
		t.Fatal("follow-up conversation was not stable")
	}
	prompt := fmt.Sprintf("Follow up on this recorded investigation. Use get_request_stats and get_security_matches for service %d from %s to %s, then use get_request to inspect request %d. Use numerals to report the exact total, blocked and allowed counts; include allowed requests with security matches. Keep it short and distinguish evidence from hypotheses.", service.ID, detail.Trigger.From.Format(time.RFC3339), detail.Trigger.To.Format(time.RFC3339), detail.Result.Evidence[0].Request.ID)
	input, _ := json.Marshal(map[string]string{"content": prompt})
	followup := call("POST", "/api/assistant/conversations/"+conversation.ID+"/messages", string(input), 200)
	var terminal assistant.Event
	tools := map[string]bool{}
	for _, frame := range strings.Split(string(followup), "\n\n") {
		for _, line := range strings.Split(frame, "\n") {
			if strings.HasPrefix(line, "data: ") {
				var e assistant.Event
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &e); err != nil {
					t.Fatal(err)
				}
				if e.Type == "tool_started" {
					tools[e.ToolName] = true
				}
				if e.Type == "run_completed" || e.Type == "run_failed" {
					terminal = e
				}
			}
		}
	}
	if terminal.Type != "run_completed" || terminal.Message == nil {
		t.Fatal("live follow-up did not complete")
	}
	for _, name := range []string{"get_request_stats", "get_security_matches", "get_request"} {
		if !tools[name] {
			t.Errorf("follow-up did not use %s", name)
		}
	}
	for _, count := range []string{"60", "30", "40"} {
		if !strings.Contains(terminal.Message.Content, count) {
			t.Errorf("follow-up omitted expected mock count %s", count)
		}
	}
	t.Logf("follow-up: tools=%v answer=%s", tools, terminal.Message.Content)
	var messages struct {
		Items []domain.AssistantMessage `json:"items"`
	}
	if err := json.Unmarshal(call("GET", "/api/assistant/conversations/"+conversation.ID+"/messages", "", 200), &messages); err != nil {
		t.Fatal(err)
	}
	if len(messages.Items) != 3 || messages.Items[2].Status != "completed" {
		t.Fatal("follow-up messages were not persisted")
	}
	if err := json.Unmarshal(call("GET", path, "", 200), &detail); err != nil {
		t.Fatal(err)
	}
	replayCtx, stopReplay := context.WithCancel(context.Background())
	defer stopReplay()
	replay := subscribeLive(t, client, replayCtx, base+path+"/events?after="+fmt.Sprint(detail.LastEventID))
	call("POST", path+"/retry", "", 202)
	started := false
	wait := time.NewTimer(30 * time.Second)
	for !started {
		select {
		case item := <-replay:
			if item.Err != nil {
				t.Fatal(item.Err)
			}
			assertPublicPayload(t, item.Raw)
			if item.Event.Type == "investigation.started" {
				started = true
			}
		case <-wait.C:
			t.Fatal("retry never started")
		}
	}
	wait.Stop()
	call("POST", path+"/cancel", "", 200)
	cancelled := false
	cancelTimeout := time.NewTimer(30 * time.Second)
	for !cancelled {
		select {
		case item := <-replay:
			if item.Err != nil {
				t.Fatal(item.Err)
			}
			assertPublicPayload(t, item.Raw)
			if item.Event.Type == "investigation.result_published" {
				t.Fatal("cancelled retry published a result")
			}
			if item.Event.Type == "investigation.cancelled" {
				cancelled = true
			}
		case <-cancelTimeout.C:
			t.Fatal("running cancellation did not finish")
		}
	}
	cancelTimeout.Stop()
	stopReplay()
	if err := json.Unmarshal(call("GET", path, "", 200), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Status != "cancelled" || detail.ResultVersion != 1 || detail.Result == nil {
		t.Fatal("cancellation did not preserve the prior result")
	}
	if err := json.Unmarshal(call("POST", path+"/follow-up", "", 200), &same); err != nil {
		t.Fatal(err)
	}
	if same.ID != conversation.ID {
		t.Fatal("retry replaced the follow-up conversation")
	}
	var runCount int64
	if err := db.Model(&domain.InvestigationRun{}).Where("incident_id = ?", investigationID).Count(&runCount).Error; err != nil {
		t.Fatal(err)
	}
	if runCount != 2 {
		t.Fatalf("expected two internal executions, got %d", runCount)
	}
	t.Log("SSE replay, running cancellation, prior-result preservation and stable follow-up passed")
}

func seedLiveTraffic(t *testing.T, db *gorm.DB, serviceID uint) map[uint64]bool {
	t.Helper()
	end := time.Now().UTC().Truncate(time.Minute)
	ids := map[uint64]bool{}
	for i := 0; i < 60; i++ {
		item := domain.RequestLog{ServiceID: &serviceID, Timestamp: end.Add(-2 * time.Minute).Add(time.Duration(i) * time.Second), Hostname: "shop.test", IP: "198.51.100.23", Method: "GET", Path: fmt.Sprintf("/catalog/item/%02d/1%%27%%20UNION%%20SELECT%%20username,password%%20FROM%%20users--", i), Action: "blocked", RuleID: "942100", Reason: "SQL Injection Attack Detected via libinjection", DurationMs: 2.5, RequestBytes: 386, ResponseBytes: 153, Status: 403, RuleMatches: []domain.RuleMatch{{RuleID: "942100", Source: "crs", Message: "SQL Injection Attack Detected via libinjection", Tags: []string{"attack-sqli"}}}}
		if i >= 30 {
			item.Action = "allowed"
			item.Status = 200
			item.DurationMs = 42
			item.ResponseBytes = 2048
		}
		if i >= 30 && i < 40 {
			item.RuleID = ""
			item.Reason = "Detection mode: request allowed"
			item.RuleMatches = append(item.RuleMatches, domain.RuleMatch{RuleID: "942100", Source: "crs", Message: "SQL Injection Attack Detected via libinjection", Variables: []string{"REQUEST_URI"}})
		}
		if i >= 40 {
			item.IP = "192.0.2.10"
			item.Path = "/products"
			item.RuleID = ""
			item.Reason = ""
			item.RuleMatches = []domain.RuleMatch{}
		}
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
		ids[item.ID] = true
	}
	return ids
}

func subscribeLive(t *testing.T, client *http.Client, ctx context.Context, url string) <-chan liveEvent {
	t.Helper()
	out := make(chan liveEvent, 512)
	go func() {
		defer close(out)
		request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			out <- liveEvent{Err: err}
			return
		}
		response, err := client.Do(request)
		if err != nil {
			if ctx.Err() == nil {
				out <- liveEvent{Err: err}
			}
			return
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			out <- liveEvent{Err: fmt.Errorf("SSE status %d", response.StatusCode)}
			return
		}
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var e domain.InvestigationEvent
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &e); err != nil {
				out <- liveEvent{Err: err}
				return
			}
			select {
			case out <- liveEvent{Event: e, Raw: []byte(strings.TrimPrefix(line, "data: "))}:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			out <- liveEvent{Err: err}
		}
	}()
	return out
}

func assertPublicPayload(t *testing.T, data []byte) {
	t.Helper()
	key := os.Getenv("AI_KEY")
	if len(key) > 8 && strings.Contains(string(data), key) {
		t.Fatal("API exposed an AI credential")
	}
	var value any
	if json.Unmarshal(data, &value) != nil {
		return
	}
	forbidden := map[string]bool{"model": true, "provider": true, "history": true, "incidentId": true, "findingId": true, "runId": true, "arguments": true}
	var inspect func(any)
	inspect = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, nested := range v {
				if forbidden[key] {
					t.Errorf("unexpected public field %s", key)
				}
				inspect(nested)
			}
		case []any:
			for _, nested := range v {
				inspect(nested)
			}
		}
	}
	inspect(value)
}
