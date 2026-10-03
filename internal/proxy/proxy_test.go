package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"OpenWAF/internal/database"
	"OpenWAF/internal/domain"
	"OpenWAF/internal/telemetry"
	"gorm.io/gorm"
)

func testService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "proxy.db"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(database.NewStore(db), telemetry.New(database.NewStore(db)), nil)
	t.Cleanup(func() {
		s.Close()
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})
	return s, db
}

func saveService(t *testing.T, db *gorm.DB, target string) domain.Service {
	t.Helper()
	service := domain.Service{Name: "Example", Hostname: "example.test", UpstreamURL: target, Enabled: true}
	if err := db.Create(&service).Error; err != nil {
		t.Fatal(err)
	}
	return service
}

func proxyRequest(s *Service, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://example.test"+path, strings.NewReader(body))
	r.Host = "EXAMPLE.TEST:8080"
	r.RemoteAddr = "192.0.2.10:12345"
	r.Header.Set("X-Forwarded-For", "spoofed")
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("Connection", "X-Hop")
	r.Header.Set("X-Hop", "remove")
	r.Header.Set("Authorization", "Bearer application-token")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestForwardingAndBlocking(t *testing.T) {
	s, db := testService(t)
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.RequestURI != "/a%2Fb?q=one&q=two&block=false" || string(body) != "request body" || r.Host != "EXAMPLE.TEST:8080" {
			t.Errorf("request changed: %s %s host=%s body=%s", r.Method, r.RequestURI, r.Host, body)
		}
		if r.Header.Get("X-Forwarded-For") != "192.0.2.10" || r.Header.Get("X-Forwarded-Host") != r.Host || r.Header.Get("X-Forwarded-Proto") != "http" || r.Header.Get("X-Hop") != "" || r.Header.Get("Authorization") != "Bearer application-token" {
			t.Errorf("unexpected upstream headers: %v", r.Header)
		}
		w.Header().Set("Set-Cookie", "application=value; Path=/")
		w.Header().Set("Location", "/next")
		w.Header().Set("Connection", "X-Upstream-Hop")
		w.Header().Set("X-Upstream-Hop", "remove")
		w.WriteHeader(307)
		fmt.Fprint(w, "upstream body")
	}))
	defer upstream.Close()
	saveService(t, db, upstream.URL)
	w := proxyRequest(s, "POST", "/a%2Fb?q=one&q=two&block=false", "request body")
	if w.Code != 307 || w.Body.String() != "upstream body" || w.Header().Get("Location") != "/next" || w.Header().Get("Set-Cookie") == "" || w.Header().Get("X-Upstream-Hop") != "" {
		t.Fatalf("response changed: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	for _, path := range []string{"/?block=true", "/?block=false&block=true", "/?%62lock=%74rue"} {
		if w := proxyRequest(s, "GET", path, ""); w.Code != 403 {
			t.Errorf("%s returned %d", path, w.Code)
		}
	}
	if w := proxyRequest(s, "GET", "/?block=%ZZ", ""); w.Code != 400 {
		t.Errorf("invalid query returned %d", w.Code)
	}
	if hits.Load() != 1 {
		t.Fatalf("blocked requests reached upstream: %d hits", hits.Load())
	}
}

func TestRoutingUpdatesAndFailures(t *testing.T) {
	s, db := testService(t)
	if w := proxyRequest(s, "GET", "/", ""); w.Code != 404 {
		t.Fatalf("unknown service: %d", w.Code)
	}
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "first") }))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "second") }))
	defer second.Close()
	service := saveService(t, db, first.URL)
	if w := proxyRequest(s, "GET", "/", ""); w.Body.String() != "first" {
		t.Fatalf("first upstream: %d %s", w.Code, w.Body.String())
	}
	if err := db.Model(&service).Update("upstream_url", second.URL).Error; err != nil {
		t.Fatal(err)
	}
	if w := proxyRequest(s, "GET", "/", ""); w.Body.String() != "second" {
		t.Fatalf("updated upstream: %d %s", w.Code, w.Body.String())
	}
	if err := db.Model(&service).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if w := proxyRequest(s, "GET", "/", ""); w.Code != 404 {
		t.Fatalf("disabled service: %d", w.Code)
	}
	if err := db.Model(&service).Update("enabled", true).Error; err != nil {
		t.Fatal(err)
	}
	second.Close()
	if w := proxyRequest(s, "GET", "/", ""); w.Code != 502 || strings.Contains(w.Body.String(), second.URL) {
		t.Fatalf("unavailable upstream: %d %s", w.Code, w.Body.String())
	}
	if err := db.Delete(&service).Error; err != nil {
		t.Fatal(err)
	}
	if w := proxyRequest(s, "GET", "/", ""); w.Code != 404 {
		t.Fatalf("deleted service: %d", w.Code)
	}
}

func TestUpstreamTLSVerification(t *testing.T) {
	s, db := testService(t)
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "secure") }))
	defer upstream.Close()
	service := saveService(t, db, upstream.URL)
	if w := proxyRequest(s, "GET", "/", ""); w.Code != 502 {
		t.Fatalf("untrusted certificate accepted: %d", w.Code)
	}
	if err := db.Model(&service).Update("skip_tls_verify", true).Error; err != nil {
		t.Fatal(err)
	}
	if w := proxyRequest(s, "GET", "/", ""); w.Code != 200 || w.Body.String() != "secure" {
		t.Fatalf("skip verification: %d %s", w.Code, w.Body.String())
	}
	if err := db.Model(&service).Update("skip_tls_verify", false).Error; err != nil {
		t.Fatal(err)
	}
	if w := proxyRequest(s, "GET", "/", ""); w.Code != 502 {
		t.Fatalf("verification change ignored: %d", w.Code)
	}
	trusted := New(database.NewStore(db), telemetry.New(database.NewStore(db)), nil)
	defer trusted.Close()
	trusted.verified.TLSClientConfig = upstream.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	if w := proxyRequest(trusted, "GET", "/", ""); w.Code != 200 {
		t.Fatalf("trusted certificate rejected: %d", w.Code)
	}
}

func TestProtocolUpgrade(t *testing.T) {
	s, db := testService(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		fmt.Fprint(rw, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
		rw.Flush()
		line, _ := rw.ReadString('\n')
		fmt.Fprint(rw, line)
		rw.Flush()
	}))
	defer upstream.Close()
	saveService(t, db, upstream.URL)
	server := httptest.NewServer(s)
	defer server.Close()
	conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprint(conn, "GET /socket HTTP/1.1\r\nHost: example.test\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n\r\n")
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: "GET"})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 101 {
		t.Fatalf("upgrade returned %d", response.StatusCode)
	}
	fmt.Fprint(conn, "ping\n")
	if line, err := reader.ReadString('\n'); err != nil || line != "ping\n" {
		t.Fatalf("upgrade stream failed: %q %v", line, err)
	}
}
