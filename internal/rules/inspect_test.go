package rules

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"OpenWAF/internal/database"
	"OpenWAF/internal/domain"
)

func TestInboundAnomalyScoreMessage(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "rules.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})
	service, err := New(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://example.test/search?q=1'+OR+'1'%3D'1", nil)
	event := &domain.RequestLog{}
	status, cleanup, err := service.Inspect(req, 0, "192.0.2.10", event)
	cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusForbidden {
		t.Fatalf("status=%d rule=%s reason=%q", status, event.RuleID, event.Reason)
	}
	if event.RuleID != "949110" && event.RuleID != "949111" {
		t.Fatalf("rule=%s reason=%q", event.RuleID, event.Reason)
	}
	if strings.Contains(event.Reason, "%{") || !regexp.MustCompile(`Total Score: [0-9]+`).MatchString(event.Reason) {
		t.Fatalf("reason=%q", event.Reason)
	}
	found := false
	for _, match := range event.RuleMatches {
		if strings.Contains(match.Message, "%{") {
			t.Errorf("match %s still has a macro: %s", match.RuleID, match.Message)
		}
		if match.RuleID == event.RuleID {
			found = match.Message == event.Reason
		}
	}
	if !found {
		t.Fatalf("blocking match message = %q, matches=%d", event.Reason, len(event.RuleMatches))
	}
}
