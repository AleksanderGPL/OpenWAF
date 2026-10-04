// Package geoip provides offline country lookups using a local DB-IP database.
package geoip

import (
	"context"
	"log"
	"net/http"
	"net/netip"
	"path/filepath"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

type Resolver struct {
	mu     sync.RWMutex
	db     *maxminddb.Reader
	path   string
	client *http.Client
	url    string
}

// Open loads cached data and fetches the current monthly release if needed.
// The resolver remains usable even on error: it uses cached data, or returns nil countries.
func Open(ctx context.Context, directory string) (*Resolver, error) {
	r := &Resolver{path: filepath.Join(directory, "dbip-country-lite.mmdb"), client: &http.Client{Timeout: 60 * time.Second}, url: "https://download.db-ip.com/free/dbip-country-lite-%s.mmdb.gz"}
	if db, err := openDatabase(r.path); err == nil {
		r.db = db
	}
	return r, r.refresh(ctx, time.Now().UTC())
}

func (r *Resolver) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.db == nil {
		return nil
	}
	err := r.db.Close()
	r.db = nil
	return err
}

// Run checks hourly for a new monthly release and retries failed downloads.
func (r *Resolver) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := r.refresh(ctx, now.UTC()); err != nil && ctx.Err() == nil {
				log.Printf("GeoIP database update failed: %v", err)
			}
		}
	}
}

// CountryCode returns nil for invalid, private, reserved, or unknown addresses.
func (r *Resolver) CountryCode(ip string) *string {
	addr, err := netip.ParseAddr(ip)
	if err != nil || addr.Zone() != "" {
		return nil
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || reserved(addr) {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.db == nil {
		return nil
	}
	var record struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}
	if err := r.db.Lookup(addr).Decode(&record); err != nil || len(record.Country.ISOCode) != 2 {
		return nil
	}
	return &record.Country.ISOCode
}

var reservedNetworks = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func reserved(addr netip.Addr) bool {
	for _, network := range reservedNetworks {
		if network.Contains(addr) {
			return true
		}
	}
	return false
}
