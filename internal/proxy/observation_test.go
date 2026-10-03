package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"OpenWAF/internal/clientip"
	"OpenWAF/internal/domain"
)

func TestRequestLogsCaptureOutcomesAndBytes(t *testing.T) {
	s, db := testService(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(500)
		fmt.Fprint(w, "upstream body")
	}))
	defer upstream.Close()
	service := saveService(t, db, upstream.URL)
	resolver, err := clientip.New([]string{"192.0.2.10"})
	if err != nil {
		t.Fatal(err)
	}
	s.clientIP = resolver
	r := httptest.NewRequest("POST", "http://example.test/a%2Fb?token=secret", strings.NewReader("request body"))
	r.RemoteAddr = "192.0.2.10:1234"
	r.Header.Set("X-Forwarded-For", "198.51.100.4")
	r.Header.Set("Authorization", "Bearer secret")
	r.Header.Set("Cookie", "secret=value")
	s.ServeHTTP(httptest.NewRecorder(), r)
	proxyRequest(s, "GET", "/?block=true", "")
	proxyRequest(s, "GET", "/?block=%ZZ", "")
	upstream.Close()
	proxyRequest(s, "GET", "/unavailable", "")
	unknown := httptest.NewRequest("GET", "http://unknown.test/missing", nil)
	s.ServeHTTP(httptest.NewRecorder(), unknown)
	var events []domain.RequestLog
	if err := db.Order("id").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 5 {
		t.Fatalf("incomplete logs: %+v", events)
	}
	first := events[0]
	if first.ServiceID == nil || *first.ServiceID != service.ID || first.Action != "allowed" || first.Status != 500 || first.ErrorCategory != "" || first.Path != "/a%2Fb" || first.IP != "198.51.100.4" || first.RequestBytes != 12 || first.ResponseBytes != 13 || first.DurationMs <= 0 {
		t.Fatalf("incorrect request log: %+v", first)
	}
	if events[1].Action != "blocked" || events[1].RuleID != "query_block" || events[1].Status != 403 || events[1].ServiceID == nil || *events[1].ServiceID != service.ID || events[1].IP != "192.0.2.10" {
		t.Fatalf("incorrect block log: %+v", events[1])
	}
	for i, want := range []string{"invalid_request", "upstream_unavailable", "service_not_found"} {
		if events[i+2].ErrorCategory != want {
			t.Fatalf("outcome %d: %+v", i+2, events[i+2])
		}
	}
	if events[4].ServiceID != nil {
		t.Fatal("unknown hostname linked to a service")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	blocked := httptest.NewRequest("GET", "http://example.test/?block=true", nil).WithContext(ctx)
	s.ServeHTTP(httptest.NewRecorder(), blocked)
	var count int64
	db.Model(&domain.RequestLog{}).Count(&count)
	if count != 6 {
		t.Fatal("disconnected request was not logged")
	}
}

type blockingRecorder struct{ entered, release chan struct{} }

func (b *blockingRecorder) Record(context.Context, *domain.RequestLog) error {
	close(b.entered)
	<-b.release
	return nil
}

func TestProxyWaitsForLogPersistence(t *testing.T) {
	s, _ := testService(t)
	recorder := &blockingRecorder{entered: make(chan struct{}), release: make(chan struct{})}
	s.recorder = recorder
	done := make(chan struct{})
	go func() { proxyRequest(s, "GET", "/?block=true", ""); close(done) }()
	select {
	case <-recorder.entered:
	case <-time.After(time.Second):
		t.Fatal("log persistence not reached")
	}
	select {
	case <-done:
		t.Fatal("proxy returned before persistence")
	default:
	}
	close(recorder.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("proxy did not return after persistence")
	}
}

func TestLoggingPreservesStreaming(t *testing.T) {
	s, db := testService(t)
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "first\n")
		w.(http.Flusher).Flush()
		<-release
		fmt.Fprint(w, "second\n")
	}))
	defer upstream.Close()
	saveService(t, db, upstream.URL)
	server := httptest.NewServer(s)
	defer server.Close()
	r, _ := http.NewRequest("GET", server.URL+"/stream", nil)
	r.Host = "example.test"
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(r)
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	defer response.Body.Close()
	first := make([]byte, 6)
	_, err = io.ReadFull(response.Body, first)
	close(release)
	if err != nil || string(first) != "first\n" {
		t.Fatalf("first stream chunk: %q %v", first, err)
	}
	rest, err := io.ReadAll(response.Body)
	if err != nil || string(rest) != "second\n" {
		t.Fatalf("second stream chunk: %q %v", rest, err)
	}
}
