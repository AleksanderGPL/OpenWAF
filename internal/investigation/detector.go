package investigation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"OpenWAF/internal/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) aggregate(ctx context.Context) (int, error) {
	processed := 0
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cursor domain.DetectorCursor
		if err := tx.First(&cursor, 1).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			var latest domain.RequestLog
			if err := tx.Order("id DESC").Limit(1).Find(&latest).Error; err != nil {
				return err
			}
			now := time.Now().UTC()
			cursor = domain.DetectorCursor{ID: 1, RequestID: latest.ID, StartedAt: now, UpdatedAt: now}
			return tx.Create(&cursor).Error
		} else if err != nil {
			return err
		}
		if time.Since(cursor.UpdatedAt) > 2*time.Minute || s.telemetry.CollectionFailures() != cursor.CollectionFailures {
			cursor.StartedAt = time.Now().UTC()
		}
		cursor.CollectionFailures = s.telemetry.CollectionFailures()
		var logs []domain.RequestLog
		if err := tx.Where("id > ?", cursor.RequestID).Order("id ASC").Limit(250).Find(&logs).Error; err != nil {
			return err
		}
		processed = len(logs)
		buckets := map[string]*domain.TrafficMinute{}
		now := time.Now().UTC()
		for _, item := range logs {
			cursor.RequestID = item.ID
			if item.ServiceID == nil || item.Timestamp.Before(now.Add(-65*time.Minute)) || item.Timestamp.After(now.Add(time.Minute)) {
				continue
			}
			minute := item.Timestamp.UTC().Truncate(time.Minute)
			key := fmt.Sprintf("%d/%d/%s", minute.Unix(), *item.ServiceID, item.IP)
			b := buckets[key]
			if b == nil {
				b = &domain.TrafficMinute{Key: key, Minute: minute, ServiceID: *item.ServiceID, IP: item.IP, Paths: []string{}, Evidence: []uint64{}}
				var old domain.TrafficMinute
				if err := tx.First(&old, "key = ?", key).Error; err == nil {
					*b = old
				} else if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				buckets[key] = b
			}
			b.Requests++
			if item.Action == "blocked" {
				b.Blocked++
			}
			suspicious := item.Action == "blocked" || len(item.RuleMatches) > 0 || oneOfSignal(item.SecuritySignals)
			if suspicious {
				b.Suspicious++
			}
			if item.Status == 404 || item.Status == 400 || item.Status == 405 {
				b.ScanFailures++
			}
			if item.Action == "allowed" && (item.Status >= 500 || item.ErrorCategory != "") {
				b.Errors++
			}
			sum := sha256.Sum256([]byte(item.Path))
			path := hex.EncodeToString(sum[:])
			if len(b.Paths) < 100 && !contains(b.Paths, path) {
				b.Paths = append(b.Paths, path)
			}
			if len(b.Evidence) < 10 {
				b.Evidence = append(b.Evidence, item.ID)
			} else if suspicious {
				b.Evidence[int(item.ID%10)] = item.ID
			}
		}
		for _, b := range buckets {
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(b).Error; err != nil {
				return err
			}
		}
		cursor.UpdatedAt = now
		return tx.Save(&cursor).Error
	})
	return processed, err
}
func contains(items []string, v string) bool {
	for _, x := range items {
		if x == v {
			return true
		}
	}
	return false
}
func oneOfSignal(items []string) bool {
	for _, x := range items {
		if oneOf(x, "high_request_rate", "repeated_blocks") {
			return true
		}
	}
	return false
}

type totals struct {
	requests, blocked, suspicious, errors, scanFailures int
	paths                                               map[string]bool
	evidence                                            []uint64
	minuteRequests                                      int
	baseline                                            int
	minuteEvidence                                      []uint64
}

func newTotals() *totals { return &totals{paths: map[string]bool{}} }

func (s *Service) detect(ctx context.Context) error {
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Where("minute < ?", now.Add(-65*time.Minute)).Delete(&domain.TrafficMinute{}).Error; err != nil {
		return err
	}
	to := now.Truncate(time.Minute)
	from := to.Add(-5 * time.Minute)
	baselineFrom := to.Add(-31 * time.Minute)
	var cursor domain.DetectorCursor
	if err := s.db.WithContext(ctx).First(&cursor, 1).Error; err != nil {
		return err
	}
	var pending []domain.RequestLog
	if err := s.db.WithContext(ctx).Model(&domain.RequestLog{}).Where("id > ? AND timestamp >= ? AND timestamp < ?", cursor.RequestID, baselineFrom, to).Select("id").Limit(1).Find(&pending).Error; err != nil {
		return err
	}
	if len(pending) > 0 {
		return nil
	}
	var buckets []domain.TrafficMinute
	if err := s.db.WithContext(ctx).Where("minute >= ? AND minute < ?", baselineFrom, to).Limit(50001).Find(&buckets).Error; err != nil {
		return err
	}
	if len(buckets) > 50000 {
		return fmt.Errorf("anomaly aggregation capacity reached; evaluation deferred")
	}
	var policies []domain.AnomalySettings
	if err := s.db.WithContext(ctx).Find(&policies).Error; err != nil {
		return err
	}
	configured := map[uint]domain.AnomalySettings{}
	for _, p := range policies {
		configured[p.Scope] = p
	}
	services := map[uint]*totals{}
	sources := map[string]*totals{}
	sourceService := map[string]uint{}
	sourceIP := map[string]string{}
	for _, b := range buckets {
		total := services[b.ServiceID]
		if total == nil {
			total = newTotals()
			services[b.ServiceID] = total
		}
		if b.Minute.Before(to.Add(-time.Minute)) {
			total.baseline += b.Requests

		} else {
			total.minuteRequests += b.Requests
			for _, id := range b.Evidence {
				if len(total.minuteEvidence) < 20 {
					total.minuteEvidence = append(total.minuteEvidence, id)
				}
			}
		}
		if b.Minute.Before(from) {
			continue
		}
		add(total, b)
		key := fmt.Sprintf("%d/%s", b.ServiceID, b.IP)
		source := sources[key]
		if source == nil {
			source = newTotals()
			sources[key] = source
			sourceService[key] = b.ServiceID
			sourceIP[key] = b.IP
		}
		add(source, b)
	}
	coverage := "Counts cover collected, persisted requests; path diversity is bounded to 100 hashes per source per minute. Late requests may revise recent buckets."
	if cursor.StartedAt.After(from) {
		coverage += " Detection is warming up; the window has partial collection coverage."
	}
	emit := func(service uint, ip, detector string, observed, threshold float64, baseline *float64, t *totals) error {
		p, ok := configured[service]
		if !ok {
			p = configured[0]
		}
		if !p.Enabled {
			return nil
		}
		trigger := domain.InvestigationTrigger{Detector: detector, From: from, To: to, Observed: observed, Threshold: threshold, Baseline: baseline, RequestIDs: t.evidence, Coverage: coverage}
		labels := map[string]string{"path_scanning": "Distinct-path scanning with security matches or request errors", "repeated_blocks": "Repeated blocked requests from this source", "repeated_security_matches": "Repeated security rule matches from this source", "error_surge": "Elevated upstream error count and ratio"}
		trigger.Explanation = fmt.Sprintf("%s: observed %.0f, meeting threshold %.0f in five minutes", labels[detector], observed, threshold)
		if detector == "path_scanning" {
			trigger.Explanation = fmt.Sprintf("This source made %d requests across at least %d paths in five minutes, with %d security matches and %d request errors; thresholds are %d requests and %d paths", t.requests, len(t.paths), t.suspicious, t.scanFailures, p.ScanRequests, p.ScanPaths)
		}
		if detector == "error_surge" {
			trigger.Explanation = fmt.Sprintf("%d upstream errors among %d requests in five minutes (%.1f%%), meeting thresholds of %d errors and %d%%", t.errors, t.requests, 100*float64(t.errors)/float64(t.requests), p.ErrorThreshold, p.ErrorPercent)
		}
		if detector == "traffic_surge" {
			trigger.From = to.Add(-time.Minute)
			trigger.RequestIDs = t.minuteEvidence
			trigger.Explanation = fmt.Sprintf("Traffic rose to %d requests in one minute; prior 30-minute baseline %.1f requests/minute, threshold %.1f", t.minuteRequests, *baseline, threshold)
		}
		return s.trigger(ctx, service, ip, trigger, p, now)
	}
	keys := make([]string, 0, len(sources))
	for key := range sources {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	emitted := 0
	for _, key := range keys {
		service := sourceService[key]
		ip := sourceIP[key]
		t := sources[key]
		p, ok := configured[service]
		if !ok {
			p = configured[0]
		}
		if !p.Enabled {
			continue
		}
		detector := ""
		observed, threshold := 0., 0.
		switch {
		case t.requests >= p.ScanRequests && len(t.paths) >= p.ScanPaths && (t.suspicious >= 3 || t.scanFailures >= 3):
			detector = "path_scanning"
			observed = float64(len(t.paths))
			threshold = float64(p.ScanPaths)
		case t.blocked >= p.BlockedThreshold:
			detector = "repeated_blocks"
			observed = float64(t.blocked)
			threshold = float64(p.BlockedThreshold)
		case t.suspicious >= p.SuspiciousThreshold:
			detector = "repeated_security_matches"
			observed = float64(t.suspicious)
			threshold = float64(p.SuspiciousThreshold)
		}
		if detector != "" {
			if err := emit(service, ip, detector, observed, threshold, nil, t); err != nil {
				return err
			}
			emitted++
			if emitted >= 100 {
				break
			}
		}
	}
	for service, t := range services {
		p, ok := configured[service]
		if !ok {
			p = configured[0]
		}
		if !p.Enabled {
			continue
		}
		if t.errors >= p.ErrorThreshold && t.requests > 0 && 100*t.errors/t.requests >= p.ErrorPercent {
			if err := emit(service, "", "error_surge", float64(t.errors), float64(p.ErrorThreshold), nil, t); err != nil {
				return err
			}
		}
		if !cursor.StartedAt.After(baselineFrom) {
			baseline := float64(t.baseline) / 30
			threshold := baseline * p.SurgeMultiplier
			if threshold < float64(p.SurgeMinimum) {
				threshold = float64(p.SurgeMinimum)
			}
			if float64(t.minuteRequests) >= threshold {
				if err := emit(service, "", "traffic_surge", float64(t.minuteRequests), threshold, &baseline, t); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func add(t *totals, b domain.TrafficMinute) {
	t.requests += b.Requests
	t.blocked += b.Blocked
	t.suspicious += b.Suspicious
	t.errors += b.Errors
	t.scanFailures += b.ScanFailures
	for _, path := range b.Paths {
		if len(t.paths) < 500 {
			t.paths[path] = true
		}
	}
	for _, id := range b.Evidence {
		if len(t.evidence) < 20 {
			t.evidence = append(t.evidence, id)
		}
	}
}

func (s *Service) trigger(ctx context.Context, service uint, ip string, trigger domain.InvestigationTrigger, p domain.AnomalySettings, now time.Time) error {
	key := fmt.Sprintf("%s/%d/%s", trigger.Detector, service, ip)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var investigation domain.Investigation
		err := tx.Where("correlation_key = ? AND last_seen >= ?", key, now.Add(-time.Duration(p.CooldownMinutes)*time.Minute)).Order("last_seen DESC").First(&investigation).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			investigation = domain.Investigation{ID: uuid.NewString(), CorrelationKey: key, ServiceID: service, IP: ip, State: "open", Trigger: trigger, FirstSeen: now, LastSeen: now}
			if err := tx.Create(&investigation).Error; err != nil {
				return err
			}
			if err := event(tx, domain.InvestigationRun{InvestigationID: investigation.ID}, "investigation.created", trigger.Explanation, ""); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			changed := !investigation.Trigger.To.Equal(trigger.To) || investigation.Trigger.Observed != trigger.Observed
			investigation.LastSeen = now
			investigation.Trigger = trigger
			if err := tx.Save(&investigation).Error; err != nil {
				return err
			}
			if changed {
				if err := event(tx, domain.InvestigationRun{InvestigationID: investigation.ID}, "investigation.updated", trigger.Explanation, ""); err != nil {
					return err
				}
			}
		}
		if oneOf(investigation.State, "resolved", "dismissed") {
			return nil
		}
		if now.Sub(investigation.LastQueuedAt) < time.Duration(p.CooldownMinutes)*time.Minute && (trigger.Observed < 2*investigation.LastQueuedValue || now.Sub(investigation.LastQueuedAt) < time.Minute) {
			return nil
		}
		_, err = queue(tx, &investigation, trigger, now, p.HourlyRunLimit)
		var fe *fiber.Error
		if errors.As(err, &fe) && (fe.Code == 429 || fe.Code == 409) {
			return nil
		}
		return err
	})
}
