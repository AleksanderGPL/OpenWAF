package telemetry

import (
	"context"
	"errors"
	"log"
	"sync/atomic"
	"time"

	"OpenWAF/internal/domain"
)

type Window struct {
	From time.Time `json:"from" required:"true"`
	To   time.Time `json:"to" required:"true"`
}

type Filter struct {
	Window
	ServiceID                          uint64
	Action, IP, Method, RuleID, Search string
	Status                             int
	Page, Limit                        int
	MaxTotal                           int64
}

type Metrics struct {
	TotalRequests    int64   `json:"totalRequests" required:"true"`
	AllowedRequests  int64   `json:"allowedRequests" required:"true"`
	BlockedRequests  int64   `json:"blockedRequests" required:"true"`
	ErrorRequests    int64   `json:"errorRequests" required:"true"`
	BlockRate        float64 `json:"blockRate" required:"true"`
	AverageLatencyMs float64 `json:"averageLatencyMs" required:"true"`
	RequestBytes     int64   `json:"requestBytes" required:"true"`
	ResponseBytes    int64   `json:"responseBytes" required:"true"`
}

type Summary struct {
	Window
	Metrics
	PreviousWindow      Window    `json:"previousWindow" required:"true"`
	Previous            Metrics   `json:"previous" required:"true"`
	CollectionFailures  uint64    `json:"collectionFailures" required:"true"`
	CollectionStartedAt time.Time `json:"collectionStartedAt" required:"true"`
	LogRetentionDays    int       `json:"logRetentionDays" required:"true"`
}

type Bucket struct {
	Timestamp time.Time `json:"timestamp" required:"true"`
	Metrics
}

type Traffic struct {
	Window
	Interval string   `json:"interval" required:"true" enum:"hour,day"`
	Buckets  []Bucket `json:"buckets" required:"true"`
}

type Threat struct {
	RuleID   string `json:"ruleId" required:"true"`
	Reason   string `json:"reason" required:"true"`
	Requests int64  `json:"requests" required:"true"`
}

type BlockedSource struct {
	IP          string    `json:"ip" required:"true"`
	CountryCode *string   `json:"countryCode" required:"true"`
	Requests    int64     `json:"requests" required:"true"`
	RuleID      string    `json:"ruleId" required:"true"`
	Reason      string    `json:"reason" required:"true"`
	LastSeen    time.Time `json:"lastSeen" required:"true"`
}

type LogPage struct {
	Items []domain.RequestLog `json:"items" required:"true"`
	Total int64               `json:"total" required:"true"`
	Page  int                 `json:"page" required:"true"`
	Limit int                 `json:"limit" required:"true"`
}

type ThreatsResponse struct {
	Window
	Items []Threat `json:"items" required:"true"`
}

type SourcesResponse struct {
	Window
	Items []BlockedSource `json:"items" required:"true"`
}

type Store interface {
	SaveRequest(context.Context, *domain.RequestLog) error
	Metrics(context.Context, Filter) (Metrics, error)
	Traffic(context.Context, Filter, string) ([]Bucket, error)
	Threats(context.Context, Filter) ([]Threat, error)
	BlockedSources(context.Context, Filter) ([]BlockedSource, error)
	Logs(context.Context, Filter) (LogPage, error)
	RequestLog(context.Context, uint64) (domain.RequestLog, error)
	Settings(context.Context) (domain.Settings, error)
	UpdateSettings(context.Context, domain.Settings) error
	PruneLogs(context.Context, time.Time) error
}

type Service struct {
	store     Store
	failures  atomic.Uint64
	startedAt time.Time
}

func New(store Store) *Service { return &Service{store: store, startedAt: time.Now().UTC()} }

func (s *Service) Record(ctx context.Context, event *domain.RequestLog) error {
	err := s.store.SaveRequest(context.WithoutCancel(ctx), event)
	if err != nil {
		s.failures.Add(1)
	}
	return err
}

func (s *Service) Summary(ctx context.Context, filter Filter) (Summary, error) {
	current, err := s.store.Metrics(ctx, filter)
	if err != nil {
		return Summary{}, err
	}
	previousFilter := filter
	previousFilter.Window = Window{From: filter.From.Add(-filter.To.Sub(filter.From)), To: filter.From}
	previous, err := s.store.Metrics(ctx, previousFilter)
	if err != nil {
		return Summary{}, err
	}
	settings, err := s.store.Settings(ctx)
	return Summary{Window: filter.Window, Metrics: current, PreviousWindow: previousFilter.Window, Previous: previous,
		CollectionFailures: s.failures.Load(), CollectionStartedAt: s.startedAt, LogRetentionDays: settings.LogRetentionDays}, err
}

func (s *Service) Traffic(ctx context.Context, filter Filter) (Traffic, error) {
	interval, step := "hour", time.Hour
	if filter.To.Sub(filter.From) > 24*time.Hour {
		interval, step = "day", 24*time.Hour
	}
	rows, err := s.store.Traffic(ctx, filter, interval)
	if err != nil {
		return Traffic{}, err
	}
	byTime := make(map[int64]Metrics, len(rows))
	for _, row := range rows {
		byTime[row.Timestamp.Unix()] = row.Metrics
	}
	buckets := make([]Bucket, 0)
	for start := filter.From.Truncate(step); start.Before(filter.To); start = start.Add(step) {
		buckets = append(buckets, Bucket{Timestamp: start, Metrics: byTime[start.Unix()]})
	}
	return Traffic{Window: filter.Window, Interval: interval, Buckets: buckets}, nil
}

func (s *Service) Settings(ctx context.Context) (domain.Settings, error) {
	return s.store.Settings(ctx)
}

func (s *Service) UpdateSettings(ctx context.Context, settings domain.Settings) (domain.Settings, error) {
	if settings.LogRetentionDays < 1 || settings.LogRetentionDays > 3650 {
		return domain.Settings{}, domain.ValidationError("logRetentionDays must be between 1 and 3650")
	}
	settings.ID = 1
	if err := s.store.UpdateSettings(ctx, settings); err != nil {
		return domain.Settings{}, err
	}
	if err := s.Prune(ctx); err != nil {
		return domain.Settings{}, err
	}
	return settings, nil
}

func (s *Service) Prune(ctx context.Context) error {
	settings, err := s.store.Settings(ctx)
	if err != nil {
		return err
	}
	return s.store.PruneLogs(ctx, time.Now().UTC().AddDate(0, 0, -settings.LogRetentionDays))
}

func (s *Service) RunRetention(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		if err := s.Prune(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("log retention cleanup failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) Threats(ctx context.Context, filter Filter) (ThreatsResponse, error) {
	items, err := s.store.Threats(ctx, filter)
	return ThreatsResponse{Window: filter.Window, Items: items}, err
}
func (s *Service) BlockedSources(ctx context.Context, filter Filter) (SourcesResponse, error) {
	items, err := s.store.BlockedSources(ctx, filter)
	return SourcesResponse{Window: filter.Window, Items: items}, err
}
func (s *Service) Logs(ctx context.Context, filter Filter) (LogPage, error) {
	return s.store.Logs(ctx, filter)
}
func (s *Service) RequestLog(ctx context.Context, id uint64) (domain.RequestLog, error) {
	return s.store.RequestLog(ctx, id)
}

func (s *Service) CollectionFailures() uint64 { return s.failures.Load() }
