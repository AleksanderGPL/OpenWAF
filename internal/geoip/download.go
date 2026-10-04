package geoip

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

func openDatabase(path string) (*maxminddb.Reader, error) {
	db, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	if err := db.Verify(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (r *Resolver) refresh(ctx context.Context, now time.Time) error {
	r.mu.RLock()
	current := r.db != nil && !r.db.Metadata.BuildTime().Before(time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC))
	r.mu.RUnlock()
	if current {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(r.url, now.Format("2006-01")), nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DB-IP download: HTTP %d", resp.StatusCode)
	}
	compressed, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("DB-IP gzip: %w", err)
	}
	defer compressed.Close()
	if err := os.MkdirAll(filepath.Dir(r.path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(r.path), ".geoip-*.mmdb")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	const maxSize = 64 << 20
	size, copyErr := io.Copy(file, io.LimitReader(compressed, maxSize+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if size > maxSize {
		return fmt.Errorf("DB-IP database exceeds %d bytes", maxSize)
	}
	db, err := openDatabase(file.Name())
	if err != nil {
		return fmt.Errorf("DB-IP database validation: %w", err)
	}
	if err := os.Rename(file.Name(), r.path); err != nil {
		db.Close()
		return err
	}
	r.mu.Lock()
	old := r.db
	r.db = db
	if old != nil {
		old.Close()
	}
	r.mu.Unlock()
	return nil
}
