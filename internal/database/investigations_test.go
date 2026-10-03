package database

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"OpenWAF/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateSplitInvestigationData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE incidents (id TEXT PRIMARY KEY, correlation_key TEXT, service_id INTEGER, ip TEXT, state TEXT, trigger TEXT, first_seen DATETIME, last_seen DATETIME, last_queued_value REAL, last_queued_at DATETIME)`,
		`CREATE TABLE findings (id TEXT PRIMARY KEY, incident_id TEXT, run_id TEXT, title TEXT, summary TEXT, severity TEXT, assessment TEXT, explanation TEXT, patterns TEXT, recommendations TEXT, limitations TEXT, evidence TEXT, trigger TEXT, state TEXT, created_at DATETIME)`,
		`CREATE TABLE finding_reads (finding_id TEXT, user_id INTEGER, read_at DATETIME, PRIMARY KEY(finding_id,user_id))`,
	}
	for _, statement := range statements {
		if err := legacy.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := legacy.AutoMigrate(&domain.InvestigationRun{}, &domain.InvestigationFollowUp{}, &domain.AssistantConversation{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	trigger := domain.InvestigationTrigger{Detector: "repeated_blocks", Explanation: "Ten blocked requests", From: now.Add(-time.Minute), To: now, Observed: 10, Threshold: 10, RequestIDs: []uint64{17}}
	triggerJSON, _ := json.Marshal(trigger)
	if err := legacy.Exec("INSERT INTO incidents (id,correlation_key,service_id,ip,state,trigger,first_seen,last_seen,last_queued_at) VALUES (?,?,?,?,?,?,?,?,?)", "legacy-investigation", "repeated_blocks/1/198.51.100.23", 1, "198.51.100.23", "open", string(triggerJSON), now.Add(-time.Minute), now, now).Error; err != nil {
		t.Fatal(err)
	}
	older := domain.InvestigationRun{ID: "older-run", InvestigationID: "legacy-investigation", Trigger: trigger, Status: "completed", CreatedAt: now.Add(-time.Minute)}
	latest := domain.InvestigationRun{ID: "latest-run", InvestigationID: "legacy-investigation", Trigger: trigger, Status: "completed", CreatedAt: now}
	for _, run := range []domain.InvestigationRun{older, latest} {
		if err := legacy.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
	}
	evidence, _ := json.Marshal([]domain.RequestLog{{ID: 17, Timestamp: now, IP: "198.51.100.23", Path: "/login", Action: "blocked", Status: 403}})
	for i, id := range []string{"older-finding", "latest-finding"} {
		run := "older-run"
		state := "open"
		title := "Old result"
		when := now.Add(-time.Minute)
		if i == 1 {
			run = "latest-run"
			state = "acknowledged"
			title = "Latest result"
			when = now
		}
		if err := legacy.Exec("INSERT INTO findings (id,incident_id,run_id,title,summary,severity,assessment,explanation,patterns,recommendations,limitations,evidence,trigger,state,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", id, "legacy-investigation", run, title, "Verified traffic evidence", "high", "likely_malicious", "Requests match an injection pattern", `["SQL injection"]`, `["Review the source"]`, `[]`, string(evidence), string(triggerJSON), state, when).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := legacy.Exec("INSERT INTO finding_reads VALUES (?,?,?)", "latest-finding", 1, now).Error; err != nil {
		t.Fatal(err)
	}
	conversation := domain.AssistantConversation{ID: "legacy-chat", UserID: 1, Title: "Prior follow-up", CreatedAt: now, UpdatedAt: now}
	if err := legacy.Create(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	if err := legacy.Create(&domain.InvestigationFollowUp{InvestigationID: "latest-run", UserID: 1, ConversationID: conversation.ID}).Error; err != nil {
		t.Fatal(err)
	}
	oldSQL, err := legacy.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := oldSQL.Close(); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		db, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		var item domain.Investigation
		if err := db.First(&item, "id = ?", "legacy-investigation").Error; err != nil {
			sqlDB.Close()
			t.Fatal(err)
		}
		if item.Result == nil || item.Result.Title != "Latest result" || item.ResultVersion != 1 || item.State != "acknowledged" || len(item.Result.Evidence) != 1 {
			sqlDB.Close()
			t.Fatalf("migration lost result/state: %+v", item)
		}
		var read domain.InvestigationRead
		if err := db.First(&read, "investigation_id = ? AND user_id = ?", item.ID, 1).Error; err != nil {
			sqlDB.Close()
			t.Fatal(err)
		}
		if read.Version != 1 {
			t.Fatalf("read version=%d", read.Version)
		}
		var link domain.InvestigationFollowUp
		if err := db.First(&link, "run_id = ? AND user_id = ?", item.ID, 1).Error; err != nil {
			sqlDB.Close()
			t.Fatal(err)
		}
		if link.ConversationID != conversation.ID {
			t.Fatal("migration replaced follow-up history")
		}
		var run domain.InvestigationRun
		if err := db.First(&run, "id = ?", "latest-run").Error; err != nil {
			sqlDB.Close()
			t.Fatal(err)
		}
		if run.Result == nil || run.Result.Title != "Latest result" {
			t.Fatal("internal execution lost its result")
		}
		var legacyCount int64
		if err := db.Table("findings").Count(&legacyCount).Error; err != nil {
			t.Fatal(err)
		}
		if legacyCount != 2 {
			t.Fatal("migration removed legacy results")
		}
		if db.Migrator().HasTable("incidents") {
			t.Fatal("legacy incident table was not renamed")
		}
		if err := sqlDB.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
