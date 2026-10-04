package investigation

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OpenWAF/internal/assistant"
	"OpenWAF/internal/database"
	"OpenWAF/internal/domain"
	"OpenWAF/internal/telemetry"
)

func TestExecutionPreservesFailureReason(t *testing.T) {
	for _, tc := range []struct {
		name      string
		oversized bool
		want      string
	}{
		{name: "context limit", oversized: true, want: "investigation exceeded context size limit"},
		{name: "provider error", want: "provider temporarily unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !tc.oversized {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusServiceUnavailable)
					fmt.Fprint(w, `{"error":{"message":"provider temporarily unavailable","type":"server_error"}}`)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"id\":\"first\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"request-call\",\"type\":\"function\",\"function\":{\"name\":\"get_request\",\"arguments\":\"{\\\"id\\\":1}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
			}))
			defer provider.Close()
			t.Setenv("AI_KEY", "test-key")
			t.Setenv("AI_MODEL", "test/model")
			t.Setenv("ASSISTANT_BASE_URL", provider.URL)
			db, err := database.Open(filepath.Join(t.TempDir(), "investigation.db"))
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			if tc.oversized {
				if err := db.Create(&domain.RequestLog{ID: 1, Timestamp: time.Now().UTC(), Reason: strings.Repeat("x", 256*1024)}).Error; err != nil {
					t.Fatal(err)
				}
			}
			ts := telemetry.New(database.NewStore(db))
			ai, err := assistant.New(context.Background(), db, ts)
			if err != nil {
				t.Fatal(err)
			}
			defer ai.Close()
			s, err := New(db, ai, ts)
			if err != nil {
				t.Fatal(err)
			}
			item := domain.Investigation{ID: "investigation", FirstSeen: time.Now().UTC(), LastSeen: time.Now().UTC()}
			if err := db.Create(&item).Error; err != nil {
				t.Fatal(err)
			}
			run := domain.InvestigationRun{ID: "run", InvestigationID: item.ID, Status: "running", Attempts: 1, CreatedAt: time.Now().UTC()}
			if err := db.Create(&run).Error; err != nil {
				t.Fatal(err)
			}
			for attempt := 1; attempt <= 2; attempt++ {
				s.execute(context.Background(), run)
				if err := db.First(&run, "id = ?", run.ID).Error; err != nil {
					t.Fatal(err)
				}
				wantStatus := "queued"
				if attempt == 2 {
					wantStatus = "failed"
				}
				if run.Status != wantStatus || !strings.Contains(run.Error, tc.want) {
					t.Fatalf("attempt %d: status=%s error=%q", attempt, run.Status, run.Error)
				}
				view, err := NewHandler(s).detail(context.Background(), 1, item.ID)
				if err != nil {
					t.Fatal(err)
				}
				if view.Error != run.Error {
					t.Fatalf("API lost reason: %q", view.Error)
				}
				var event domain.InvestigationEvent
				if err := db.Where("run_id = ? AND type = ?", run.ID, "investigation."+wantStatus).Order("id DESC").First(&event).Error; err != nil {
					t.Fatal(err)
				}
				if event.Activity != run.Error {
					t.Fatalf("event lost reason: %q", event.Activity)
				}
				if attempt == 1 {
					run.Status = "running"
					run.Attempts++
					if err := db.Save(&run).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
