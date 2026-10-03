package domain

import "time"

type RequestLog struct {
	ID            uint64    `gorm:"primaryKey" json:"id" required:"true"`
	Timestamp     time.Time `gorm:"not null;index:idx_logs_time;index:idx_logs_service_time,priority:2;index:idx_logs_action_time,priority:2;index:idx_logs_ip_time,priority:2" json:"timestamp" required:"true"`
	ServiceID     *uint     `gorm:"index:idx_logs_service_time,priority:1" json:"serviceId" required:"true"`
	Hostname      string    `gorm:"not null" json:"hostname" required:"true"`
	IP            string    `gorm:"not null;index:idx_logs_ip_time,priority:1" json:"ip" required:"true"`
	CountryCode   *string   `json:"countryCode" required:"true"`
	Method        string    `gorm:"not null" json:"method" required:"true"`
	Path          string    `gorm:"not null" json:"path" required:"true"`
	Action        string    `gorm:"not null;index:idx_logs_action_time,priority:1" json:"action" required:"true" enum:"allowed,blocked"`
	RuleID        string    `gorm:"not null" json:"ruleId" required:"true"`
	Reason        string    `gorm:"not null" json:"reason" required:"true"`
	Status        int       `gorm:"not null" json:"status" required:"true"`
	ErrorCategory string    `gorm:"not null" json:"errorCategory" required:"true"`
	DurationMs    float64   `gorm:"not null" json:"durationMs" required:"true"`
	RequestBytes  int64     `gorm:"not null" json:"requestBytes" required:"true"`
	ResponseBytes int64     `gorm:"not null" json:"responseBytes" required:"true"`
}

type Settings struct {
	ID               uint `gorm:"primaryKey;check:id = 1" json:"-"`
	LogRetentionDays int  `gorm:"not null;check:log_retention_days BETWEEN 1 AND 3650" json:"logRetentionDays" required:"true" minimum:"1" maximum:"3650"`
}
