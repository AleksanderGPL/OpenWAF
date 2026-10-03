package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OpenWAF/internal/api"
	"OpenWAF/internal/auth"
	"OpenWAF/internal/database"
	"OpenWAF/internal/domain"
	"OpenWAF/internal/telemetry"
	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"
)

// Opt in explicitly: this test makes paid model requests with synthetic telemetry.
func TestLiveAssistant(t *testing.T) {
	if os.Getenv("OPENWAF_LIVE_AI") != "1" {
		t.Skip("set OPENWAF_LIVE_AI=1 to test the real provider")
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
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	store := database.NewStore(db)
	s, err := New(context.Background(), db, telemetry.New(store))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	authService, err := auth.New(store)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{ErrorHandler: api.ErrorHandler})
	defer app.Shutdown()
	router := api.New(app).Group("/api")
	authHandler := auth.NewHandler(authService, false, nil)
	authHandler.Register(router)
	NewHandler(s).Register(router, authHandler.RequireAuth)
	now := time.Now().UTC()
	for i := 0; i < 12; i++ {
		event := domain.RequestLog{Timestamp: now.Add(-time.Duration(i+1) * time.Minute), Hostname: "shop.test", IP: "198.51.100.23", Method: "GET", Path: "/login", Action: "blocked", RuleID: "test-sqli", Reason: "Synthetic SQL injection rule match", Status: 403}
		if i >= 8 {
			event.IP = "192.0.2.10"
			event.Path = "/products"
			event.Action = "allowed"
			event.RuleID = ""
			event.Reason = ""
			event.Status = 200
		}
		if err := db.Create(&event).Error; err != nil {
			t.Fatal(err)
		}
	}
	var cookies []*http.Cookie
	call := func(method, path, body string, want int) string {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		response, err := app.Test(req, fiber.TestConfig{Timeout: 210 * time.Second})
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if len(response.Cookies()) > 0 {
			cookies = response.Cookies()
		}
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s: status %d want %d: %s", method, path, response.StatusCode, want, data)
		}
		return string(data)
	}
	call("GET", "/api/assistant/status", "", 401)
	call("POST", "/api/auth/setup", `{"username":"live-test","password":"isolated-test-password"}`, 201)
	call("POST", "/api/auth/sign-in", `{"username":"live-test","password":"isolated-test-password"}`, 200)
	call("GET", "/api/assistant/status", "", 200)
	var conversation domain.AssistantConversation
	if err := json.Unmarshal([]byte(call("POST", "/api/assistant/conversations", `{"title":"Live provider test"}`, 201)), &conversation); err != nil {
		t.Fatal(err)
	}
	path := "/api/assistant/conversations/" + conversation.ID + "/messages"
	prompts := []string{
		"Investigate the last hour using get_request_stats and search_requests. Report total, blocked and allowed requests, identify suspicious source IPs, cite request IDs, and distinguish evidence from hypotheses. Keep the answer short.",
		"Follow up on that suspicious IP from your previous answer. Use get_request to inspect one specific request you cited, and get_threats to verify the rule pattern. Cite the request ID and explain the evidence briefly.",
	}
	for i, prompt := range prompts {
		input, _ := json.Marshal(messageRequest{Content: prompt})
		stream := call("POST", path, string(input), 200)
		events := make(map[string]int)
		tools := make(map[string]bool)
		var terminal Event
		for _, frame := range strings.Split(stream, "\n\n") {
			for _, line := range strings.Split(frame, "\n") {
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				var event Event
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
					t.Fatal(err)
				}
				events[event.Type]++
				if event.Type == "tool_started" {
					tools[event.ToolName] = true
				}
				if event.Type == "run_completed" || event.Type == "run_failed" {
					terminal = event
				}
			}
		}
		if terminal.Type != "run_completed" || terminal.Message == nil {
			t.Fatalf("turn %d failed: %+v", i+1, terminal)
		}
		if events["text_delta"] == 0 || events["tool_finished"] == 0 {
			t.Fatalf("missing text/tools: %v", events)
		}
		expected := []string{"get_request_stats", "search_requests"}
		if i == 1 {
			expected = []string{"get_request", "get_threats"}
		}
		for _, name := range expected {
			if !tools[name] {
				t.Errorf("turn %d did not call requested tool %s", i+1, name)
			}
		}
		if !strings.Contains(terminal.Message.Content, "198.51.100.23") {
			t.Errorf("turn %d did not identify the seeded suspicious IP", i+1)
		}
		t.Logf("turn %d: events=%v tools=%v answer=%s", i+1, events, tools, terminal.Message.Content)
	}
	var history messageList
	if err := json.Unmarshal([]byte(call("GET", path, "", 200)), &history); err != nil {
		t.Fatal(err)
	}
	if len(history.Items) != 4 {
		t.Fatalf("expected four persisted messages, got %d", len(history.Items))
	}
	for _, message := range history.Items {
		if message.Status != "completed" {
			t.Fatalf("unexpected persisted status: %s", message.Status)
		}
	}
	call("DELETE", fmt.Sprintf("/api/assistant/conversations/%s", conversation.ID), "", 200)
}
