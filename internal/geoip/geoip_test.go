package geoip

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// tinyDatabase synthesizes an MMDB with one node mapping public IPs to a country.
// It contains no downloaded geographic data.
func tinyDatabase(country string, built time.Time) []byte {
	str := func(s string) []byte { return append([]byte{0x40 | byte(len(s))}, s...) }
	data := []byte{0, 0, 17, 0, 0, 17}
	data = append(data, make([]byte, 16)...)
	data = append(data, 0xe1)
	data = append(data, str("country")...)
	data = append(data, 0xe1)
	data = append(data, str("iso_code")...)
	data = append(data, str(country)...)
	data = append(data, []byte("\xab\xcd\xefMaxMind.com")...)
	data = append(data, 0xe9)
	field := func(name string, value []byte) { data = append(data, str(name)...); data = append(data, value...) }
	field("node_count", []byte{0xc1, 1})
	field("record_size", []byte{0xa1, 24})
	field("ip_version", []byte{0xa1, 6})
	field("binary_format_major_version", []byte{0xa1, 2})
	field("binary_format_minor_version", []byte{0xa0})
	epoch := binary.BigEndian.AppendUint64([]byte{8, 2}, uint64(built.Unix()))
	field("build_epoch", epoch)
	field("database_type", str("DBIP-Country-Lite"))
	field("languages", append([]byte{1, 4}, str("en")...))
	field("description", append(append([]byte{0xe1}, str("en")...), str("Test database")...))
	return data
}

func TestRefreshAndOfflineLookup(t *testing.T) {
	now := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "country.mmdb")
	old := tinyDatabase("GB", now.AddDate(0, -2, 0))
	if err := os.WriteFile(path, old, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := openDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	r := &Resolver{db: db, path: path, url: "https://example.test/%s.mmdb.gz"}
	t.Cleanup(func() { r.Close() })
	calls := 0
	r.client = &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Path != "/"+now.Format("2006-01")+".mmdb.gz" {
			t.Fatal(req.URL)
		}
		var body bytes.Buffer
		w := gzip.NewWriter(&body)
		w.Write(tinyDatabase("US", now))
		w.Close()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(&body)}, nil
	})}
	if got := r.CountryCode("8.8.8.8"); got == nil || *got != "GB" {
		t.Fatalf("cached country: %v", got)
	}
	if err := r.refresh(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"8.8.8.8", "::ffff:8.8.8.8", "2001:4860:4860::8888"} {
		if got := r.CountryCode(ip); got == nil || *got != "US" {
			t.Fatalf("%s: %v", ip, got)
		}
	}
	for _, ip := range []string{"invalid", "127.0.0.1", "10.0.0.1", "192.168.1.1", "100.64.0.1", "192.0.2.1", "198.51.100.1", "203.0.113.1", "::1", "fc00::1", "2001:db8::1", "ff02::1"} {
		if got := r.CountryCode(ip); got != nil {
			t.Fatalf("%s: unexpected %s", ip, *got)
		}
	}
	if err := r.refresh(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("downloads: %d", calls)
	}
	reopened, err := openDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	reopened.Close()
}

func TestFailedDownloadPreservesCache(t *testing.T) {
	for _, status := range []int{200, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "country.mmdb")
			original := tinyDatabase("GB", time.Now().AddDate(0, -2, 0))
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			db, err := openDatabase(path)
			if err != nil {
				t.Fatal(err)
			}
			r := &Resolver{db: db, path: path, url: "https://example.test/%s"}
			t.Cleanup(func() { r.Close() })
			r.client = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				var body bytes.Buffer
				w := gzip.NewWriter(&body)
				w.Write([]byte("invalid database"))
				w.Close()
				return &http.Response{StatusCode: status, Body: io.NopCloser(&body)}, nil
			})}
			if err := r.refresh(context.Background(), time.Now().UTC()); err == nil {
				t.Fatal("expected download error")
			}
			if got := r.CountryCode("8.8.8.8"); got == nil || *got != "GB" {
				t.Fatalf("lost cached country: %v", got)
			}
			cached, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(cached, original) {
				t.Fatalf("cache changed: %v", err)
			}
			files, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".geoip-*"))
			if len(files) != 0 {
				t.Fatalf("temporary files remain: %v", files)
			}
		})
	}
}
