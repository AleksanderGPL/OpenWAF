package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OpenWAF/internal/api"
	"OpenWAF/internal/database"
	"OpenWAF/internal/domain"
	"OpenWAF/internal/telemetry"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gofiber/fiber/v3"
)

type testAgent struct {
	inputs chan []*schema.Message
	wait   bool
}

func (a *testAgent) Name(context.Context) string        { return "test" }
func (a *testAgent) Description(context.Context) string { return "test" }
func (a *testAgent) Run(ctx context.Context, input *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		a.inputs <- input.Messages
		if a.wait {
			<-ctx.Done()
			gen.Send(&adk.AgentEvent{Err: ctx.Err()})
			return
		}
		gen.Send(adk.EventFromMessage(schema.AssistantMessage("Checking evidence.", []schema.ToolCall{{ID: "call-1", Function: schema.FunctionCall{Name: "search_requests", Arguments: `{"limit":1}`}}}), nil, schema.Assistant, ""))
		gen.Send(adk.EventFromMessage(schema.ToolMessage(`{"items":[{"id":42}]}`, "call-1"), nil, schema.Tool, "search_requests"))
		stream := schema.StreamReaderFromArray([]*schema.Message{{Role: schema.Assistant, Content: "Request 42 "}, {Role: schema.Assistant, Content: "is evidence."}})
		gen.Send(adk.EventFromMessage(nil, stream, schema.Assistant, ""))
	}()
	return iter
}

func fixture(t *testing.T) (*Service, *fiber.App, *testAgent) {
	t.Helper()
	t.Setenv("AI_KEY", "")
	t.Setenv("AI_MODEL", "")
	db, err := database.Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(context.Background(), db, telemetry.New(database.NewStore(db)))
	if err != nil {
		t.Fatal(err)
	}
	a := &testAgent{inputs: make(chan []*schema.Message, 10)}
	s.runner = adk.NewRunner(context.Background(), adk.RunnerConfig{Agent: a, EnableStreaming: true})
	app := fiber.New(fiber.Config{ErrorHandler: api.ErrorHandler})
	NewHandler(s).Register(api.New(app).Group("/api"), func(c fiber.Ctx) error {
		id, role := uint(1), "admin"
		switch c.Get("X-Test-User") {
		case "none":
			return fiber.ErrUnauthorized
		case "other":
			id = 2
		case "user":
			role = "user"
		}
		c.Locals("authUser", domain.User{ID: id, Role: role})
		return c.Next()
	})
	t.Cleanup(func() { s.Close(); app.Shutdown(); sqlDB, _ := db.DB(); sqlDB.Close() })
	return s, app, a
}
func request(t *testing.T, app *fiber.App, method, path, body, user string, want int) string {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", user)
	res, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != want {
		t.Fatalf("status %d want %d: %s", res.StatusCode, want, b)
	}
	return string(b)
}
func create(t *testing.T, app *fiber.App) string {
	t.Helper()
	body := request(t, app, "POST", "/api/assistant/conversations", `{"title":"Spike"}`, "", 201)
	var c domain.AssistantConversation
	if err := json.Unmarshal([]byte(body), &c); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func TestStreamPersistsToolHistoryAndOwnership(t *testing.T) {
	s, app, a := fixture(t)
	id := create(t, app)
	path := "/api/assistant/conversations/" + id
	request(t, app, "GET", path+"/messages", "", "other", 404)
	request(t, app, "GET", path+"/messages", "", "user", 403)
	request(t, app, "GET", path+"/messages", "", "none", 401)
	body := request(t, app, "POST", path+"/messages", `{"content":"Investigate"}`, "", 200)
	for _, event := range []string{"run_started", "text_delta", "tool_started", "tool_finished", "run_completed"} {
		if !strings.Contains(body, "event: "+event+"\n") {
			t.Fatalf("missing %s: %s", event, body)
		}
	}
	messages, err := s.messages(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[1].Content != "Checking evidence.\n\nRequest 42 is evidence." || messages[1].Status != "completed" {
		t.Fatalf("messages: %+v", messages)
	}
	<-a.inputs
	request(t, app, "POST", path+"/messages", `{"content":"What about that IP?"}`, "", 200)
	history := <-a.inputs
	found := false
	for _, message := range history {
		if message.Role == schema.Tool && message.ToolCallID == "call-1" {
			found = true
		}
	}
	if !found {
		t.Fatal("follow-up lost tool evidence")
	}
	request(t, app, "DELETE", path, "", "other", 404)
	request(t, app, "DELETE", path, "", "", 200)
	messages, err = s.messages(context.Background(), id)
	if err != nil || len(messages) != 0 {
		t.Fatalf("cascade failed: %v %+v", err, messages)
	}
}

func TestCancelAndConcurrentRun(t *testing.T) {
	s, app, a := fixture(t)
	a.wait = true
	id := create(t, app)
	turn, err := s.begin(1, id, "Investigate")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan []Event, 1)
	go func() {
		var events []Event
		s.Execute(turn, func(e Event) error { events = append(events, e); return nil })
		done <- events
	}()
	<-a.inputs
	request(t, app, "POST", "/api/assistant/conversations/"+id+"/messages", `{"content":"Again"}`, "", 409)
	request(t, app, "DELETE", "/api/assistant/conversations/"+id, "", "", 409)
	request(t, app, "POST", "/api/assistant/conversations/"+id+"/cancel", "", "other", 404)
	request(t, app, "POST", "/api/assistant/conversations/"+id+"/cancel", "", "", 200)
	events := <-done
	last := events[len(events)-1]
	if last.Type != "run_failed" || last.Message.Status != "cancelled" {
		t.Fatalf("terminal: %+v", last)
	}
	messages, err := s.messages(context.Background(), id)
	if err != nil || messages[1].Status != "cancelled" {
		t.Fatalf("persisted: %+v %v", messages, err)
	}
	turn, err = s.begin(1, id, "Try again")
	if err != nil {
		t.Fatal(err)
	}
	if len(turn.history) != 1 {
		t.Fatal("failed turn entered follow-up context")
	}
	turn.cancel()
	s.Execute(turn, func(Event) error { return context.Canceled })
}

func TestDisabledAndQueryBounds(t *testing.T) {
	s, app, _ := fixture(t)
	s.runner = nil
	id := create(t, app)
	request(t, app, "POST", "/api/assistant/conversations/"+id+"/messages", `{"content":"Investigate"}`, "", 503)
	for _, q := range []query{{Limit: 101}, {Page: -1}, {From: "2026-01-01T00:00:00Z", To: "2026-03-01T00:00:00Z"}, {IP: "invalid"}, {Action: "delete"}, {Status: 999}} {
		if _, err := q.filter(); err == nil {
			t.Fatalf("accepted invalid query: %+v", q)
		}
	}
}

func TestProviderToolRoundTrip(t *testing.T) {
	s, app, _ := fixture(t)
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected provider request: %s", r.URL.Path)
		}
		var body struct {
			Tools    []json.RawMessage `json:"tools"`
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		expectedTools := map[string]bool{"get_request_stats": true, "get_traffic": true, "get_threats": true, "get_blocked_sources": true, "search_requests": true, "get_request": true, "get_security_matches": true, "get_minute_traffic": true}
		for _, raw := range body.Tools {
			var definition struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			if err := json.Unmarshal(raw, &definition); err != nil {
				t.Error(err)
			}
			if !expectedTools[definition.Function.Name] {
				t.Errorf("unexpected or duplicate tool: %s", definition.Function.Name)
			}
			delete(expectedTools, definition.Function.Name)
		}
		for name := range expectedTools {
			t.Errorf("missing tool: %s", name)
		}
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		if calls == 1 {
			fmt.Fprint(w, "data: {\"id\":\"first\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"stats-call\",\"type\":\"function\",\"function\":{\"name\":\"get_request_stats\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
		} else {
			found := false
			for _, message := range body.Messages {
				if message.Role == "tool" {
					found = true
				}
			}
			if !found {
				t.Error("model did not receive tool output")
			}
			fmt.Fprint(w, "data: {\"id\":\"second\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"No retained requests found.\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
		}
	}))
	defer provider.Close()
	t.Setenv("AI_KEY", "test-key")
	t.Setenv("AI_MODEL", "test/model")
	t.Setenv("ASSISTANT_BASE_URL", provider.URL+"/api/v1")
	configured, err := New(context.Background(), s.db, telemetry.New(database.NewStore(s.db)))
	if err != nil {
		t.Fatal(err)
	}
	s.runner = configured.runner
	id := create(t, app)
	body := request(t, app, "POST", "/api/assistant/conversations/"+id+"/messages", `{"content":"Show traffic stats"}`, "", 200)
	if !strings.Contains(body, "event: run_completed") || !strings.Contains(body, "No retained requests found.") || !strings.Contains(body, "get_request_stats") {
		t.Fatalf("stream: %s", body)
	}
}
