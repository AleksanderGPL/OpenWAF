package domain

import "time"

type RuleCondition struct {
	Target   string `json:"target" enum:"path,query,header,method,ip"`
	Key      string `json:"key,omitempty"`
	Operator string `json:"operator" enum:"equals,contains,prefix,suffix,regex,cidr"`
	Value    string `json:"value"`
}

type Rule struct {
	ID          uint            `gorm:"primaryKey" json:"id"`
	ServiceID   *uint           `gorm:"index" json:"serviceId"`
	Service     *Service        `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Name        string          `gorm:"not null" json:"name"`
	Description string          `json:"description"`
	Enabled     bool            `gorm:"not null" json:"enabled"`
	Action      string          `gorm:"not null" json:"action" enum:"block,log"`
	Conditions  []RuleCondition `gorm:"serializer:json;not null" json:"conditions"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type RulePolicy struct {
	Scope                  uint      `gorm:"primaryKey;autoIncrement:false" json:"-"`
	ServiceID              *uint     `gorm:"uniqueIndex" json:"serviceId"`
	Service                *Service  `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	Mode                   string    `gorm:"not null" json:"mode" enum:"blocking,detection,off"`
	BlockingParanoiaLevel  int       `gorm:"not null" json:"blockingParanoiaLevel"`
	DetectionParanoiaLevel int       `gorm:"not null" json:"detectionParanoiaLevel"`
	InboundThreshold       int       `gorm:"not null" json:"inboundThreshold"`
	MaxBodyBytes           int64     `gorm:"not null" json:"maxBodyBytes"`
	RateLimitPerMinute     int       `gorm:"not null" json:"rateLimitPerMinute"`
	RateLimitAction        string    `gorm:"not null" json:"rateLimitAction" enum:"block,log"`
	DisabledRuleIDs        []int     `gorm:"serializer:json;not null" json:"disabledRuleIds"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type RuleMatch struct {
	RuleID    string   `json:"ruleId"`
	Source    string   `json:"source"`
	Message   string   `json:"message"`
	Severity  string   `json:"severity,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Variables []string `json:"variables,omitempty"`
}
