package domain

import "time"

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id" required:"true"`
	Username     string    `gorm:"not null;uniqueIndex" json:"username" required:"true"`
	Name         string    `gorm:"not null" json:"name" required:"true"`
	PasswordHash string    `gorm:"not null" json:"-"`
	Role         string    `gorm:"not null;check:role IN ('admin','user')" json:"role" required:"true" enum:"admin,user"`
	CreatedAt    time.Time `json:"createdAt" required:"true"`
}

type UserSession struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null;index"`
	User      User      `gorm:"constraint:OnDelete:CASCADE"`
	TokenHash string    `gorm:"not null;uniqueIndex"`
	ExpiresAt time.Time `gorm:"not null;index"`
	CreatedAt time.Time
}

type Service struct {
	ID            uint      `gorm:"primaryKey" json:"id" required:"true"`
	Name          string    `gorm:"not null" json:"name" required:"true"`
	Hostname      string    `gorm:"not null;uniqueIndex" json:"hostname" required:"true"`
	UpstreamURL   string    `gorm:"not null" json:"upstreamUrl" required:"true"`
	SkipTLSVerify bool      `gorm:"not null;default:false" json:"skipTlsVerify" required:"true"`
	Enabled       bool      `gorm:"not null" json:"enabled" required:"true"`
	CreatedAt     time.Time `json:"createdAt" required:"true"`
	UpdatedAt     time.Time `json:"updatedAt" required:"true"`
}
