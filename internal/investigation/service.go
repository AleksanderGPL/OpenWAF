package investigation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"OpenWAF/internal/assistant"
	"OpenWAF/internal/domain"
	"OpenWAF/internal/telemetry"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db        *gorm.DB
	assistant *assistant.Service
	telemetry *telemetry.Service
	mu        sync.Mutex
	active    map[string]context.CancelFunc
}

func Defaults() domain.AnomalySettings {
	return domain.AnomalySettings{Enabled: true, BlockedThreshold: 10, SuspiciousThreshold: 10, ScanRequests: 30, ScanPaths: 20, ErrorThreshold: 20, ErrorPercent: 50, SurgeMinimum: 100, SurgeMultiplier: 4, CooldownMinutes: 10, HourlyRunLimit: 20}
}

func New(db *gorm.DB, ai *assistant.Service, ts *telemetry.Service) (*Service, error) {
	s := &Service{db: db, assistant: ai, telemetry: ts, active: map[string]context.CancelFunc{}}
	settings := Defaults()
	if err := db.Where("scope = ?", 0).Attrs(settings).FirstOrCreate(&domain.AnomalySettings{}).Error; err != nil {
		return nil, err
	}
	return s, nil
}

func event(tx *gorm.DB, run domain.InvestigationRun, kind, activity, delta string, activityIDs ...string) error {
	activityID := ""
	if len(activityIDs) > 0 {
		activityID = activityIDs[0]
	}
	return tx.Create(&domain.InvestigationEvent{RunID: run.ID, InvestigationID: run.InvestigationID, Type: kind, ActivityID: activityID, Attempt: run.Attempts, Provisional: kind == "explanation.delta", Activity: activity, Delta: delta, CreatedAt: time.Now().UTC()}).Error
}

func (s *Service) policy(ctx context.Context, scope uint) (domain.AnomalySettings, error) {
	var p domain.AnomalySettings
	err := s.db.WithContext(ctx).First(&p, "scope = ?", scope).Error
	if errors.Is(err, gorm.ErrRecordNotFound) && scope != 0 {
		err = s.db.WithContext(ctx).First(&p, "scope = ?", 0).Error
	}
	return p, err
}

func queue(tx *gorm.DB, investigation *domain.Investigation, trigger domain.InvestigationTrigger, now time.Time, limit int) (domain.InvestigationRun, error) {
	var active, hourly int64
	if err := tx.Model(&domain.InvestigationRun{}).Where("incident_id = ? AND status IN ?", investigation.ID, []string{"queued", "running"}).Count(&active).Error; err != nil {
		return domain.InvestigationRun{}, err
	}
	if active > 0 {
		return domain.InvestigationRun{}, fiber.NewError(409, "An investigation is already active")
	}
	if err := tx.Model(&domain.InvestigationRun{}).Where("status IN ?", []string{"queued", "running"}).Count(&active).Error; err != nil {
		return domain.InvestigationRun{}, err
	}
	if err := tx.Model(&domain.InvestigationRun{}).Where("created_at >= ?", now.Add(-time.Hour)).Count(&hourly).Error; err != nil {
		return domain.InvestigationRun{}, err
	}
	var global domain.AnomalySettings
	if err := tx.First(&global, "scope = ?", 0).Error; err != nil {
		return domain.InvestigationRun{}, err
	}
	var serviceHourly int64
	if err := tx.Model(&domain.InvestigationRun{}).Joins("JOIN investigations ON investigations.id = investigation_runs.incident_id").Where("investigation_runs.created_at >= ? AND investigations.service_id = ?", now.Add(-time.Hour), investigation.ServiceID).Count(&serviceHourly).Error; err != nil {
		return domain.InvestigationRun{}, err
	}
	if active >= 100 || hourly >= int64(global.HourlyRunLimit) || serviceHourly >= int64(limit) {
		return domain.InvestigationRun{}, fiber.NewError(429, "Investigation allowance reached")
	}
	run := domain.InvestigationRun{ID: uuid.NewString(), InvestigationID: investigation.ID, Trigger: trigger, Status: "queued", CreatedAt: now, AvailableAt: now}
	if err := tx.Create(&run).Error; err != nil {
		return run, err
	}
	if err := tx.Model(investigation).Updates(map[string]any{"last_queued_at": now, "last_queued_value": trigger.Observed}).Error; err != nil {
		return run, err
	}
	return run, event(tx, run, "investigation.queued", "Waiting to investigate", "")
}

func (s *Service) Run(ctx context.Context) {
	done := make(chan struct{})
	go func() { defer close(done); s.worker(ctx) }()
	defer func() {
		s.mu.Lock()
		for _, cancel := range s.active {
			cancel()
		}
		s.mu.Unlock()
		<-done
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	nextEvaluation := time.Time{}
	for {
		deadline := time.Now().Add(500 * time.Millisecond)
		for {
			n, err := s.aggregate(ctx)
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("anomaly aggregation: %v", err)
				}
				break
			}
			if n < 250 || time.Now().After(deadline) {
				break
			}
		}
		if !time.Now().Before(nextEvaluation) {
			if err := s.detect(ctx); err != nil && ctx.Err() == nil {
				log.Printf("anomaly detection: %v", err)
			}
			nextEvaluation = time.Now().Add(15 * time.Second)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}

}

func (s *Service) worker(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if s.assistant.CanInvestigate() {
			run, err := s.claim(ctx)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && ctx.Err() == nil {
				log.Printf("claim investigation: %v", err)
			}
			if err == nil {
				s.execute(ctx, run)
				continue
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) claim(ctx context.Context) (domain.InvestigationRun, error) {
	var run domain.InvestigationRun
	now := time.Now().UTC()
	found := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var expired []domain.InvestigationRun
		if err := tx.Where("status = ? AND lease_until < ?", "running", now).Limit(100).Find(&expired).Error; err != nil {
			return err
		}
		for _, old := range expired {
			status := "queued"
			if old.Attempts >= 2 {
				status = "failed"
			}
			if old.CancelRequested {
				status = "cancelled"
			}
			changes := map[string]any{"status": status, "lease_until": nil, "available_at": now, "error": "Previous investigation was interrupted"}
			kind := "investigation.queued"
			if status != "queued" {
				changes["finished_at"] = now
				kind = "investigation." + status
			}
			if err := tx.Model(&old).Updates(changes).Error; err != nil {
				return err
			}
			if err := event(tx, old, kind, "Recovering interrupted investigation", ""); err != nil {
				return err
			}
		}
		var global domain.AnomalySettings
		if err := tx.First(&global, "scope = ?", 0).Error; err != nil {
			return err
		}
		var attempts int64
		if err := tx.Model(&domain.InvestigationEvent{}).Where("type = ? AND created_at >= ?", "investigation.started", now.Add(-time.Hour)).Count(&attempts).Error; err != nil {
			return err
		}
		if attempts >= int64(global.HourlyRunLimit) {
			return nil
		}
		if err := tx.Where("status = ? AND available_at <= ? AND cancel_requested = ?", "queued", now, false).Order("created_at ASC").Limit(1).Find(&run).Error; err != nil {
			return err
		}
		if run.ID == "" {
			return nil
		}
		found = true
		until := now.Add(5 * time.Minute)
		run.Status = "running"
		run.Attempts++
		run.StartedAt = &now
		run.LeaseUntil = &until
		if err := tx.Save(&run).Error; err != nil {
			return err
		}
		return event(tx, run, "investigation.started", "Examining the detected activity", "")
	})
	if err == nil && !found {
		err = gorm.ErrRecordNotFound
	}
	return run, err
}

func validateReport(r assistant.Report) error {
	if len(strings.TrimSpace(r.Title)) == 0 || len(r.Title) > 160 || len(strings.TrimSpace(r.Summary)) == 0 || len(r.Summary) > 2000 || len(strings.TrimSpace(r.Explanation)) == 0 || len(r.Explanation) > 20000 {
		return fmt.Errorf("title, summary and explanation must be nonempty and within their size limits")
	}
	if !oneOf(r.Severity, "info", "low", "medium", "high", "critical") || !oneOf(r.Assessment, "likely_malicious", "likely_benign", "inconclusive") {
		return fmt.Errorf("invalid severity or assessment")
	}
	if len(r.RequestIDs) > 20 || (len(r.RequestIDs) == 0 && r.Assessment != "inconclusive") {
		return fmt.Errorf("cite up to 20 observed request IDs; an empty evidence list requires an inconclusive assessment")
	}
	if len(r.RequestIDs) == 0 && len(r.Limitations) == 0 {
		return fmt.Errorf("explain the evidence limitations for an inconclusive finding without request evidence")
	}
	if len(r.Recommendations) > 10 || len(r.Limitations) > 10 || len(r.Patterns) > 10 {
		return fmt.Errorf("too many recommendations or limitations")
	}
	for _, text := range append(append(append([]string{}, r.Recommendations...), r.Limitations...), r.Patterns...) {
		if len(text) > 2000 {
			return fmt.Errorf("recommendation or limitation too long")
		}
	}
	return nil
}
func oneOf(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}

func (s *Service) execute(parent context.Context, run domain.InvestigationRun) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	s.mu.Lock()
	s.active[run.ID] = cancel
	s.mu.Unlock()
	defer func() { cancel(); s.mu.Lock(); delete(s.active, run.ID); s.mu.Unlock() }()
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				var current domain.InvestigationRun
				if err := s.db.WithContext(ctx).Select("cancel_requested").First(&current, "id = ?", run.ID).Error; err == nil && current.CancelRequested {
					cancel()
					return
				}
			}
		}
	}()
	var investigation domain.Investigation
	err := s.db.WithContext(ctx).First(&investigation, "id = ?", run.InvestigationID).Error
	var report assistant.Report
	var history string
	if err == nil {
		var pending strings.Builder
		lastFlush := time.Now()
		flush := func() error {
			if pending.Len() == 0 {
				return nil
			}
			delta := pending.String()
			pending.Reset()
			lastFlush = time.Now()
			return event(s.db.WithContext(ctx), run, "explanation.delta", "", delta)
		}
		brief, _ := json.Marshal(struct {
			ServiceID uint                        `json:"serviceId"`
			IP        string                      `json:"ip"`
			Trigger   domain.InvestigationTrigger `json:"trigger"`
		}{investigation.ServiceID, investigation.IP, run.Trigger})
		report, history, err = s.assistant.Investigate(ctx, s.telemetry, string(brief), validateReport, func(e assistant.Event) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			kind, activity, delta := "", "", ""
			switch e.Type {
			case "text_delta":
				pending.WriteString(e.Delta)
				if pending.Len() >= 512 || time.Since(lastFlush) >= 250*time.Millisecond {
					return flush()
				}
				return nil
			case "tool_started":
				kind = "activity.started"
				activity = activityLabel(e.ToolName)
			case "tool_finished":
				kind = "activity.completed"
				activity = activityLabel(e.ToolName)
			}
			if err := flush(); err != nil {
				return err
			}
			if kind == "" {
				return nil
			}
			return event(s.db.WithContext(ctx), run, kind, activity, delta, e.ToolCallID)
		})
	}
	cancelled := ctx.Err() != nil
	cancel()
	<-watchDone
	persistCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	saveErr := s.db.WithContext(persistCtx).Transaction(func(tx *gorm.DB) error {
		var current domain.InvestigationRun
		if e := tx.First(&current, "id = ?", run.ID).Error; e != nil {
			return e
		}
		if current.Status != "running" {
			return nil
		}
		now := time.Now().UTC()
		current.LeaseUntil = nil
		if current.CancelRequested {
			current.Status = "cancelled"
			current.Error = "Investigation cancelled"
		} else if err != nil || cancelled {
			current.Status = "failed"
			current.Error = "Investigation could not complete"
			if err != nil {
				current.Error = err.Error()
			} else if ctx.Err() != nil {
				current.Error = ctx.Err().Error()
			}
			if parent.Err() != nil {
				current.Error = "Investigation interrupted by server shutdown"
			}
		} else {
			evidence, e := observedEvidence(tx, report.RequestIDs, history)
			if e != nil {
				current.Status = "failed"
				current.Error = fmt.Sprintf("Investigation evidence could not be verified: %v", e)
			} else {
				result := domain.InvestigationResult{Title: report.Title, Summary: report.Summary, Severity: report.Severity, Assessment: report.Assessment, Explanation: report.Explanation, Patterns: report.Patterns, Recommendations: report.Recommendations, Limitations: report.Limitations, Evidence: evidence, Trigger: run.Trigger, PublishedAt: now}
				var target domain.Investigation
				if e := tx.First(&target, "id = ?", run.InvestigationID).Error; e != nil {
					return e
				}
				target.Result = &result
				target.ResultVersion++
				if e := tx.Save(&target).Error; e != nil {
					return e
				}
				current.Status = "completed"
				current.Result = &result
				current.History = history
				current.Error = ""
				if e := event(tx, current, "investigation.result_published", "Investigation result available", ""); e != nil {
					return e
				}
			}
		}
		if current.Status == "failed" && current.Attempts < 2 {
			current.Status = "queued"
			current.AvailableAt = now.Add(time.Minute)
		} else {
			current.FinishedAt = &now
		}
		if e := tx.Save(&current).Error; e != nil {
			return e
		}
		return event(tx, current, "investigation."+current.Status, current.Error, "")
	})
	if err != nil {
		log.Printf("investigation %s: %v", run.ID, err)
	}
	if saveErr != nil {
		log.Printf("persist investigation %s: %v", run.ID, saveErr)
	}
}

func observedEvidence(tx *gorm.DB, ids []uint64, history string) ([]domain.RequestLog, error) {
	var messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(history), &messages); err != nil {
		return nil, err
	}
	observed := map[uint64]bool{}
	var collect func(any)
	collect = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if _, ok := x["timestamp"]; ok {
				if raw, ok := x["id"].(json.Number); ok {
					if id, err := strconv.ParseUint(raw.String(), 10, 64); err == nil {
						observed[id] = true
					}
				}
			}
			for _, nested := range x {
				collect(nested)
			}
		case []any:
			for _, nested := range x {
				collect(nested)
			}
		}
	}
	for _, m := range messages {
		if m.Role == "tool" {
			var v any
			decoder := json.NewDecoder(strings.NewReader(m.Content))
			decoder.UseNumber()
			if decoder.Decode(&v) == nil {
				collect(v)
			}
		}
	}
	evidence := make([]domain.RequestLog, 0, len(ids))
	seen := map[uint64]bool{}
	for _, id := range ids {
		if !observed[id] {
			return nil, fmt.Errorf("request %d was not observed by the agent", id)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		var item domain.RequestLog
		if err := tx.First(&item, "id = ?", id).Error; err != nil {
			return nil, err
		}
		item = boundedSnapshot(item)
		evidence = append(evidence, item)
	}
	return evidence, nil
}

func activityLabel(name string) string {
	switch name {
	case "get_request_stats":
		return "Comparing request statistics"
	case "get_traffic":
		return "Examining traffic over time"
	case "get_threats":
		return "Examining security rule matches"
	case "get_blocked_sources":
		return "Checking suspicious sources"
	case "search_requests":
		return "Searching relevant requests"
	case "get_request":
		return "Inspecting request evidence"
	case "get_security_matches":
		return "Comparing security rule matches"
	case "get_minute_traffic":
		return "Comparing recent traffic minute by minute"
	case "submit_finding":
		return "Preparing the investigation result"
	default:
		return "Examining traffic evidence"
	}
}

func (s *Service) Cancel(ctx context.Context, id string) error {
	runID := ""
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item domain.Investigation
		if err := tx.Select("id").First(&item, "id = ?", id).Error; err != nil {
			return notFound(err)
		}
		var run domain.InvestigationRun
		err := tx.Where("incident_id = ? AND status IN ?", id, []string{"queued", "running"}).Order("created_at DESC, id DESC").First(&run).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		runID = run.ID
		run.CancelRequested = true
		kind, activity := "investigation.cancellation_requested", "Cancellation requested"
		if run.Status == "queued" {
			now := time.Now().UTC()
			run.Status = "cancelled"
			run.Error = "Investigation cancelled"
			run.FinishedAt = &now
			kind = "investigation.cancelled"
			activity = "Investigation cancelled"
		}
		if err := tx.Save(&run).Error; err != nil {
			return err
		}
		return event(tx, run, kind, activity, "")
	})
	if err == nil && runID != "" {
		s.mu.Lock()
		if cancel := s.active[runID]; cancel != nil {
			cancel()
		}
		s.mu.Unlock()
	}
	return err
}

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.ErrNotFound
	}
	return err
}

func boundedSnapshot(item domain.RequestLog) domain.RequestLog {
	item.Path = bound(item.Path, 1024)
	item.Reason = bound(item.Reason, 512)
	item.Hostname = bound(item.Hostname, 253)
	if len(item.RuleMatches) > 5 {
		item.RuleMatches = item.RuleMatches[:5]
	}
	for i := range item.RuleMatches {
		m := &item.RuleMatches[i]
		m.Message = bound(m.Message, 256)
		if len(m.Tags) > 3 {
			m.Tags = m.Tags[:3]
		}
		if len(m.Variables) > 3 {
			m.Variables = m.Variables[:3]
		}
		for j := range m.Tags {
			m.Tags[j] = bound(m.Tags[j], 64)
		}
		for j := range m.Variables {
			m.Variables[j] = bound(m.Variables[j], 64)
		}
	}
	return item
}
func bound(value string, n int) string {
	if len(value) > n {
		return value[:n]
	}
	return value
}
