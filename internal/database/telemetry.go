package database

import (
	"context"
	"errors"
	"time"

	"OpenWAF/internal/domain"
	"OpenWAF/internal/telemetry"
	"gorm.io/gorm"
)

func (s *Store) SaveRequest(ctx context.Context, event *domain.RequestLog) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if event.ServiceID == nil {
			var service domain.Service
			if err := tx.Select("id").Where("hostname = ? AND enabled = ?", event.Hostname, true).Limit(1).Find(&service).Error; err != nil {
				return err
			}
			if service.ID != 0 {
				event.ServiceID = &service.ID
			}
		}
		return tx.Create(event).Error
	})
}

func (s *Store) logQuery(ctx context.Context, f telemetry.Filter) *gorm.DB {
	q := s.db.WithContext(ctx).Model(&domain.RequestLog{}).Where("timestamp >= ? AND timestamp < ?", f.From, f.To)
	if f.ServiceID != 0 {
		q = q.Where("service_id = ?", f.ServiceID)
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.IP != "" {
		q = q.Where("ip = ?", f.IP)
	}
	if f.Method != "" {
		q = q.Where("method = ?", f.Method)
	}
	if f.RuleID != "" {
		q = q.Where("rule_id = ?", f.RuleID)
	}
	if f.Status != 0 {
		q = q.Where("status = ?", f.Status)
	}
	if f.Search != "" {
		q = q.Where("instr(lower(path), lower(?)) > 0 OR instr(lower(hostname), lower(?)) > 0 OR instr(lower(ip), lower(?)) > 0 OR instr(lower(reason), lower(?)) > 0 OR CAST(id AS TEXT) = ?", f.Search, f.Search, f.Search, f.Search, f.Search)
	}
	return q
}

const metricColumns = `COUNT(*) AS total_requests,
 COALESCE(SUM(action = 'allowed'), 0) AS allowed_requests,
 COALESCE(SUM(action = 'blocked'), 0) AS blocked_requests,
 COALESCE(SUM(error_category != '' OR (action = 'allowed' AND status >= 500)), 0) AS error_requests,
 COALESCE(AVG(duration_ms), 0) AS average_latency_ms,
 COALESCE(SUM(request_bytes), 0) AS request_bytes,
 COALESCE(SUM(response_bytes), 0) AS response_bytes`

func blockRate(metrics *telemetry.Metrics) {
	if metrics.TotalRequests > 0 {
		metrics.BlockRate = float64(metrics.BlockedRequests) / float64(metrics.TotalRequests) * 100
	}
}

func (s *Store) Metrics(ctx context.Context, filter telemetry.Filter) (telemetry.Metrics, error) {
	var metrics telemetry.Metrics
	err := s.logQuery(ctx, filter).Select(metricColumns).Scan(&metrics).Error
	blockRate(&metrics)
	return metrics, err
}

func (s *Store) Traffic(ctx context.Context, filter telemetry.Filter, interval string) ([]telemetry.Bucket, error) {
	format := "%Y-%m-%dT%H:00:00Z"
	if interval == "day" {
		format = "%Y-%m-%dT00:00:00Z"
	}
	var rows []struct {
		Bucket string
		telemetry.Metrics
	}
	err := s.logQuery(ctx, filter).Select("strftime(?, timestamp) AS bucket, "+metricColumns, format).Group("bucket").Order("bucket").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	buckets := make([]telemetry.Bucket, 0, len(rows))
	for _, row := range rows {
		timestamp, err := time.Parse(time.RFC3339, row.Bucket)
		if err != nil {
			return nil, err
		}
		blockRate(&row.Metrics)
		buckets = append(buckets, telemetry.Bucket{Timestamp: timestamp, Metrics: row.Metrics})
	}
	return buckets, nil
}

func (s *Store) Threats(ctx context.Context, filter telemetry.Filter) ([]telemetry.Threat, error) {
	rows := make([]telemetry.Threat, 0)
	err := s.logQuery(ctx, filter).Where("action = ?", "blocked").Select("rule_id, reason, COUNT(*) AS requests").Group("rule_id, reason").Order("requests DESC, rule_id ASC").Scan(&rows).Error
	return rows, err
}

func (s *Store) BlockedSources(ctx context.Context, filter telemetry.Filter) ([]telemetry.BlockedSource, error) {
	var groups []struct {
		IP          string
		CountryCode *string
		Requests    int64
	}
	err := s.logQuery(ctx, filter).Where("action = ?", "blocked").Select("ip, MAX(country_code) AS country_code, COUNT(*) AS requests").Group("ip").Order("requests DESC, ip ASC").Limit(10).Scan(&groups).Error
	if err != nil {
		return nil, err
	}
	result := make([]telemetry.BlockedSource, 0, len(groups))
	for _, group := range groups {
		var rule telemetry.Threat
		if err := s.logQuery(ctx, filter).Where("action = ? AND ip = ?", "blocked", group.IP).Select("rule_id, reason, COUNT(*) AS requests").Group("rule_id, reason").Order("requests DESC, rule_id ASC").Limit(1).Scan(&rule).Error; err != nil {
			return nil, err
		}
		var latest domain.RequestLog
		if err := s.logQuery(ctx, filter).Where("action = ? AND ip = ?", "blocked", group.IP).Order("timestamp DESC, id DESC").First(&latest).Error; err != nil {
			return nil, err
		}
		result = append(result, telemetry.BlockedSource{IP: group.IP, CountryCode: group.CountryCode, Requests: group.Requests, RuleID: rule.RuleID, Reason: rule.Reason, LastSeen: latest.Timestamp})
	}
	return result, nil
}

func (s *Store) Countries(ctx context.Context, filter telemetry.Filter) ([]telemetry.CountryRequests, error) {
	rows := make([]telemetry.CountryRequests, 0)
	const country = "NULLIF(UPPER(TRIM(country_code)), '')"
	err := s.logQuery(ctx, filter).Select(country + " AS country_code, COUNT(*) AS requests").Group(country).Order("requests DESC, " + country + " IS NULL ASC, " + country + " ASC").Scan(&rows).Error
	return rows, err
}

func (s *Store) Logs(ctx context.Context, filter telemetry.Filter) (telemetry.LogPage, error) {
	page := telemetry.LogPage{Items: make([]domain.RequestLog, 0), Page: filter.Page, Limit: filter.Limit}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		store := NewStore(tx)
		if err := store.logQuery(ctx, filter).Count(&page.Total).Error; err != nil {
			return err
		}
		if filter.MaxTotal > 0 && page.Total > filter.MaxTotal {
			return nil
		}
		return store.logQuery(ctx, filter).Order("timestamp DESC, id DESC").Limit(filter.Limit).Offset((filter.Page - 1) * filter.Limit).Find(&page.Items).Error
	})
	return page, err
}

func (s *Store) RequestLog(ctx context.Context, id uint64) (domain.RequestLog, error) {
	var event domain.RequestLog
	err := s.db.WithContext(ctx).First(&event, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = domain.ErrLogNotFound
	}
	return event, err
}

func (s *Store) Settings(ctx context.Context) (domain.Settings, error) {
	var settings domain.Settings
	err := s.db.WithContext(ctx).First(&settings, 1).Error
	return settings, err
}

func (s *Store) UpdateSettings(ctx context.Context, settings domain.Settings) error {
	return s.db.WithContext(ctx).Model(&domain.Settings{}).Where("id = 1").Update("log_retention_days", settings.LogRetentionDays).Error
}

func (s *Store) PruneLogs(ctx context.Context, cutoff time.Time) error {
	for {
		result := s.db.WithContext(ctx).Exec("DELETE FROM request_logs WHERE id IN (SELECT id FROM request_logs WHERE timestamp < ? ORDER BY timestamp LIMIT 1000)", cutoff)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected < 1000 {
			return nil
		}
	}
}
