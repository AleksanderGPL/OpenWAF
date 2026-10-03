package database

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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

func Open(path string) (*gorm.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	file, err := os.OpenFile(absolute, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("create database file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	dsn := (&url.URL{Scheme: "file", Path: absolute}).String() + "?_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL&_txlock=immediate"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&User{}, &UserSession{}, &Service{}); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return db, nil
}
