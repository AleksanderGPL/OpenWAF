package domain

import "time"

type AnomalySettings struct {
	Scope               uint    `gorm:"primaryKey;autoIncrement:false" json:"scope"`
	Enabled             bool    `json:"enabled"`
	BlockedThreshold    int     `json:"blockedThreshold"`
	SuspiciousThreshold int     `json:"suspiciousThreshold"`
	ScanRequests        int     `json:"scanRequests"`
	ScanPaths           int     `json:"scanPaths"`
	ErrorThreshold      int     `json:"errorThreshold"`
	ErrorPercent        int     `json:"errorPercent"`
	SurgeMinimum        int     `json:"surgeMinimum"`
	SurgeMultiplier     float64 `json:"surgeMultiplier"`
	CooldownMinutes     int     `json:"cooldownMinutes"`
	HourlyRunLimit      int     `json:"hourlyRunLimit"`
}

type DetectorCursor struct {
	ID                 uint `gorm:"primaryKey"`
	RequestID          uint64
	StartedAt          time.Time
	UpdatedAt          time.Time
	CollectionFailures uint64
}

type TrafficMinute struct {
	Key          string    `gorm:"primaryKey"`
	Minute       time.Time `gorm:"index"`
	ServiceID    uint      `gorm:"index"`
	IP           string
	Requests     int
	Blocked      int
	Suspicious   int
	Errors       int
	ScanFailures int
	Paths        []string `gorm:"serializer:json"`
	Evidence     []uint64 `gorm:"serializer:json"`
}

type InvestigationTrigger struct {
	Detector    string    `json:"detector"`
	Explanation string    `json:"explanation"`
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	Observed    float64   `json:"observed"`
	Threshold   float64   `json:"threshold"`
	Baseline    *float64  `json:"baseline,omitempty"`
	RequestIDs  []uint64  `json:"requestIds"`
	Coverage    string    `json:"coverage"`
}

type Investigation struct {
	Result          *InvestigationResult `gorm:"serializer:json" json:"result,omitempty"`
	ResultVersion   uint64               `gorm:"not null;default:0" json:"resultVersion"`
	ID              string               `gorm:"primaryKey" json:"id"`
	CorrelationKey  string               `gorm:"index" json:"-"`
	ServiceID       uint                 `gorm:"index" json:"serviceId"`
	IP              string               `json:"ip,omitempty"`
	State           string               `json:"state"`
	Trigger         InvestigationTrigger `gorm:"serializer:json" json:"trigger"`
	FirstSeen       time.Time            `json:"firstSeen"`
	LastSeen        time.Time            `json:"lastSeen"`
	LastQueuedValue float64              `json:"-"`
	LastQueuedAt    time.Time            `json:"-"`
}

type InvestigationRun struct {
	ID              string               `gorm:"primaryKey" json:"id"`
	InvestigationID string               `gorm:"column:incident_id;index" json:"-"`
	Trigger         InvestigationTrigger `gorm:"serializer:json" json:"trigger"`
	Status          string               `gorm:"index" json:"status"`
	Attempts        int                  `json:"attempts"`
	CreatedAt       time.Time            `gorm:"index" json:"createdAt"`
	StartedAt       *time.Time           `json:"startedAt,omitempty"`
	FinishedAt      *time.Time           `json:"finishedAt,omitempty"`
	AvailableAt     time.Time            `json:"-"`
	LeaseUntil      *time.Time           `json:"-"`
	CancelRequested bool                 `json:"cancellationRequested"`
	Error           string               `json:"error,omitempty"`
	Result          *InvestigationResult `gorm:"serializer:json" json:"-"`
	History         string               `json:"-"`
}

type InvestigationResult struct {
	Title           string               `json:"title"`
	Summary         string               `json:"summary"`
	Severity        string               `json:"severity"`
	Assessment      string               `json:"assessment"`
	Explanation     string               `json:"explanation"`
	Patterns        []string             `json:"patterns"`
	Recommendations []string             `json:"recommendations"`
	Limitations     []string             `json:"limitations"`
	Evidence        []RequestLog         `json:"evidence"`
	Trigger         InvestigationTrigger `json:"trigger"`
	PublishedAt     time.Time            `json:"publishedAt"`
}

type InvestigationRead struct {
	InvestigationID string `gorm:"primaryKey"`
	UserID          uint   `gorm:"primaryKey"`
	Version         uint64
	ReadAt          time.Time
}

type InvestigationEvent struct {
	ID              uint64    `gorm:"primaryKey" json:"id"`
	RunID           string    `gorm:"index" json:"-"`
	InvestigationID string    `gorm:"column:incident_id;index" json:"investigationId"`
	Type            string    `gorm:"index:idx_investigation_event_type_time,priority:1" json:"type"`
	ActivityID      string    `json:"activityId,omitempty"`
	Attempt         int       `json:"attempt"`
	Provisional     bool      `json:"provisional"`
	Activity        string    `json:"activity,omitempty"`
	Delta           string    `json:"delta,omitempty"`
	CreatedAt       time.Time `gorm:"index:idx_investigation_event_type_time,priority:2" json:"createdAt"`
}

type InvestigationFollowUp struct {
	InvestigationID string `gorm:"column:run_id;primaryKey"`
	UserID          uint   `gorm:"primaryKey"`
	ConversationID  string
}
