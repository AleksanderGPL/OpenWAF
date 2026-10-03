package telemetry_test

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OpenWAF/internal/api"
	"OpenWAF/internal/auth"
	"OpenWAF/internal/database"
	"OpenWAF/internal/domain"
	"OpenWAF/internal/telemetry"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

type fixture struct {
	app         *fiber.App
	db          *gorm.DB
	service     *telemetry.Service
	admin, user *http.Cookie
	path        string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "logs.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	store := database.NewStore(db)
	service := telemetry.New(store)
	authService, err := auth.New(store)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{ErrorHandler: api.ErrorHandler})
	t.Cleanup(func() { app.Shutdown() })
	handler := auth.NewHandler(authService, false, nil)
	telemetry.NewHandler(service).Register(api.New(app).Group("/api"), handler.RequireAuth)
	cookies := make([]*http.Cookie, 0, 2)
	for i, role := range []string{"admin", "user"} {
		user := domain.User{Username: role, Name: role, Role: role, PasswordHash: "unused"}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		token := strings.Repeat(fmt.Sprint(i+1), 64)
		hash := sha256.Sum256([]byte(token))
		session := domain.UserSession{UserID: user.ID, TokenHash: hex.EncodeToString(hash[:]), ExpiresAt: time.Now().Add(time.Hour)}
		if err := db.Create(&session).Error; err != nil {
			t.Fatal(err)
		}
		cookies = append(cookies, &http.Cookie{Name: "session", Value: token})
	}
	return fixture{app: app, db: db, service: service, admin: cookies[0], user: cookies[1], path: path}
}

func request(t *testing.T, f fixture, method, path, body string, status int, cookie *http.Cookie) (*http.Response, []byte) {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	response, err := f.app.Test(r, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != status {
		t.Fatalf("%s %s: status=%d want=%d body=%s err=%v", method, path, response.StatusCode, status, data, err)
	}
	return response, data
}

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestTelemetryAuthorization(t *testing.T) {
	f := newFixture(t)
	for _, path := range []string{"/api/stats", "/api/stats/traffic", "/api/stats/threats", "/api/stats/blocked-sources", "/api/logs", "/api/logs/export", "/api/logs/1", "/api/settings"} {
		request(t, f, "GET", path, "", 401, nil)
		request(t, f, "GET", path, "", 403, f.user)
	}
	request(t, f, "PUT", "/api/settings", `{"logRetentionDays":1}`, 401, nil)
	request(t, f, "PUT", "/api/settings", `{"logRetentionDays":1}`, 403, f.user)
	_, body := request(t, f, "GET", "/api/stats", "", 200, f.admin)
	empty := decode[telemetry.Summary](t, body)
	if empty.TotalRequests != 0 || empty.BlockRate != 0 || empty.AverageLatencyMs != 0 || empty.LogRetentionDays != 30 {
		t.Fatalf("empty summary: %+v", empty)
	}
	_, body = request(t, f, "GET", "/api/logs", "", 200, f.admin)
	if page := decode[telemetry.LogPage](t, body); page.Items == nil || page.Total != 0 {
		t.Fatalf("empty logs: %+v", page)
	}
}

func TestStatsLogsAndExport(t *testing.T) {
	f := newFixture(t)
	services := []domain.Service{{Name: "A", Hostname: "a.test", UpstreamURL: "http://127.0.0.1", Enabled: true}, {Name: "B", Hostname: "b.test", UpstreamURL: "http://127.0.0.1", Enabled: true}}
	if err := f.db.Create(&services).Error; err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	to := from.Add(2 * time.Hour)
	events := []domain.RequestLog{
		{Timestamp: from, ServiceID: &services[0].ID, Hostname: "a.test", IP: "192.0.2.1", Method: "POST", Path: "/first", Action: "allowed", Reason: "Passed all rules", Status: 200, DurationMs: 10, RequestBytes: 3, ResponseBytes: 5},
		{Timestamp: from.Add(10 * time.Minute), Hostname: "a.test", IP: "192.0.2.2", Method: "GET", Path: "/blocked", Action: "blocked", RuleID: "query_block", Reason: "Query block check", Status: 403, DurationMs: 20, ResponseBytes: 16},
		{Timestamp: from.Add(70 * time.Minute), ServiceID: &services[0].ID, Hostname: "a.test", IP: "192.0.2.2", Method: "GET", Path: "/blocked", Action: "blocked", RuleID: "query_block", Reason: "Query block check", Status: 403, DurationMs: 30, ResponseBytes: 16},
		{Timestamp: from.Add(80 * time.Minute), ServiceID: &services[0].ID, Hostname: "a.test", IP: "192.0.2.1", Method: "POST", Path: "/broken", Action: "allowed", Status: 500, DurationMs: 40, RequestBytes: 5, ResponseBytes: 8},
		{Timestamp: from.Add(30 * time.Minute), ServiceID: &services[1].ID, Hostname: "b.test", IP: "192.0.2.3", Method: "GET", Path: "/other", Action: "allowed", Status: 200, DurationMs: 5},
		{Timestamp: from.Add(-time.Hour), ServiceID: &services[0].ID, Hostname: "a.test", IP: "192.0.2.1", Method: "GET", Path: "/previous", Action: "allowed", Status: 200, DurationMs: 50},
		{Timestamp: to, ServiceID: &services[0].ID, Hostname: "a.test", IP: "192.0.2.1", Method: "GET", Path: "/excluded", Action: "allowed", Status: 200},
	}
	for i := range events {
		if err := f.service.Record(context.Background(), &events[i]); err != nil {
			t.Fatal(err)
		}
	}
	query := "?from=" + url.QueryEscape(from.Format(time.RFC3339)) + "&to=" + url.QueryEscape(to.Format(time.RFC3339)) + fmt.Sprintf("&serviceId=%d", services[0].ID)
	response, body := request(t, f, "GET", "/api/stats"+query, "", 200, f.admin)
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("stats can be cached")
	}
	stats := decode[telemetry.Summary](t, body)
	if stats.TotalRequests != 4 || stats.AllowedRequests != 2 || stats.BlockedRequests != 2 || stats.BlockRate != 50 || stats.ErrorRequests != 1 || stats.AverageLatencyMs != 25 || stats.RequestBytes != 8 || stats.ResponseBytes != 45 || stats.Previous.TotalRequests != 1 || stats.Previous.AverageLatencyMs != 50 {
		t.Fatalf("stats: %+v", stats)
	}
	_, body = request(t, f, "GET", "/api/stats/traffic"+query, "", 200, f.admin)
	traffic := decode[telemetry.Traffic](t, body)
	if traffic.Interval != "hour" || len(traffic.Buckets) != 2 || traffic.Buckets[0].TotalRequests != 2 || traffic.Buckets[1].AverageLatencyMs != 35 {
		t.Fatalf("traffic: %+v", traffic)
	}
	_, body = request(t, f, "GET", "/api/stats/threats"+query, "", 200, f.admin)
	threats := decode[telemetry.ThreatsResponse](t, body)
	if len(threats.Items) != 1 || threats.Items[0].RuleID != "query_block" || threats.Items[0].Requests != 2 {
		t.Fatalf("threats: %+v", threats)
	}
	_, body = request(t, f, "GET", "/api/stats/blocked-sources"+query, "", 200, f.admin)
	sources := decode[telemetry.SourcesResponse](t, body)
	if len(sources.Items) != 1 || sources.Items[0].IP != "192.0.2.2" || sources.Items[0].Requests != 2 || !sources.Items[0].LastSeen.Equal(events[2].Timestamp) {
		t.Fatalf("sources: %+v", sources)
	}
	_, body = request(t, f, "GET", "/api/logs"+query+"&limit=1&page=2", "", 200, f.admin)
	page := decode[telemetry.LogPage](t, body)
	if page.Total != 4 || len(page.Items) != 1 || page.Items[0].ID != events[2].ID || page.Page != 2 {
		t.Fatalf("page: %+v", page)
	}
	_, body = request(t, f, "GET", "/api/logs"+query+"&action=blocked&ip=192.0.2.2&method=get&status=403&ruleId=query_block&search=blocked", "", 200, f.admin)
	if page := decode[telemetry.LogPage](t, body); page.Total != 2 {
		t.Fatalf("filters: %+v", page)
	}
	_, body = request(t, f, "GET", "/api/logs"+query+"&search="+url.QueryEscape("' OR 1=1 --"), "", 200, f.admin)
	if page := decode[telemetry.LogPage](t, body); page.Total != 0 {
		t.Fatalf("search is not literal: %+v", page)
	}
	_, body = request(t, f, "GET", fmt.Sprintf("/api/logs/%d", events[0].ID), "", 200, f.admin)
	if event := decode[domain.RequestLog](t, body); event.ID != events[0].ID {
		t.Fatalf("detail: %+v", event)
	}
	request(t, f, "GET", "/api/logs/99999", "", 404, f.admin)
	response, body = request(t, f, "GET", "/api/logs/export"+query+"&action=blocked", "", 200, f.admin)
	rows, err := csv.NewReader(strings.NewReader(string(body))).ReadAll()
	if err != nil || len(rows) != 3 || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/csv") {
		t.Fatalf("export: rows=%v err=%v", rows, err)
	}
	if err := f.db.Delete(&services[0]).Error; err != nil {
		t.Fatal(err)
	}
	_, body = request(t, f, "GET", "/api/logs"+query, "", 200, f.admin)
	if page := decode[telemetry.LogPage](t, body); page.Total != 4 {
		t.Fatal("deleting service deleted its historical logs")
	}
	dailyFilter := telemetry.Filter{Window: telemetry.Window{From: from, To: from.Add(7 * 24 * time.Hour)}}
	daily, err := f.service.Traffic(context.Background(), dailyFilter)
	if err != nil || daily.Interval != "day" || len(daily.Buckets) != 8 || daily.Buckets[0].TotalRequests != 6 || daily.Buckets[1].TotalRequests != 0 {
		t.Fatalf("daily traffic: %+v err=%v", daily, err)
	}
}

func TestFilterValidation(t *testing.T) {
	f := newFixture(t)
	for _, query := range []string{"range=1d", "serviceId=0", "serviceId=no", "from=2026-01-01T00:00:00Z", "from=invalid&to=invalid", "from=2026-01-02T00:00:00Z&to=2026-01-01T00:00:00Z", "from=2026-01-01T00:00:00Z&to=2026-01-01T00:00:00Z", "from=2020-01-01T00:00:00Z&to=2026-01-01T00:00:00Z", "from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z&range=24h", "action=challenged", "ip=spoofed", "page=-1", "limit=101", "status=600"} {
		request(t, f, "GET", "/api/logs?"+query, "", 400, f.admin)
	}
	request(t, f, "GET", "/api/logs/zero", "", 400, f.admin)
	request(t, f, "GET", "/api/logs/0", "", 400, f.admin)
	for _, period := range []string{"24h", "7d", "30d"} {
		request(t, f, "GET", "/api/stats?range="+period, "", 200, f.admin)
	}
}

func TestRetentionSettingsPersistenceAndCleanup(t *testing.T) {
	f := newFixture(t)
	now := time.Now().UTC()
	for _, age := range []time.Duration{40 * 24 * time.Hour, 2 * 24 * time.Hour, time.Hour} {
		event := domain.RequestLog{Timestamp: now.Add(-age), Action: "allowed", Status: 200, IP: "192.0.2.1", Path: "/"}
		if err := f.service.Record(context.Background(), &event); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.service.Prune(context.Background()); err != nil {
		t.Fatal(err)
	}
	var count int64
	f.db.Model(&domain.RequestLog{}).Count(&count)
	if count != 2 {
		t.Fatalf("default retention count=%d", count)
	}
	for _, body := range []string{`{}`, `null`, `{`, `{"logRetentionDays":0}`, `{"logRetentionDays":3651}`} {
		request(t, f, "PUT", "/api/settings", body, 400, f.admin)
	}
	_, body := request(t, f, "PUT", "/api/settings", `{"logRetentionDays":1}`, 200, f.admin)
	if settings := decode[domain.Settings](t, body); settings.LogRetentionDays != 1 {
		t.Fatal("settings not updated")
	}
	f.db.Model(&domain.RequestLog{}).Count(&count)
	if count != 1 {
		t.Fatalf("shortened retention count=%d", count)
	}
	reopened, err := database.Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := reopened.DB()
	defer sqlDB.Close()
	settings, err := database.NewStore(reopened).Settings(context.Background())
	if err != nil || settings.LogRetentionDays != 1 {
		t.Fatalf("settings lost after restart: %+v %v", settings, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { f.service.RunRetention(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("retention worker did not stop")
	}
}

func TestCanceledRequestsStillPersistAndFailuresAreReported(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	event := domain.RequestLog{Timestamp: time.Now().UTC(), Action: "allowed", Status: 200, Path: "/"}
	if err := f.service.Record(ctx, &event); err != nil || event.ID == 0 {
		t.Fatalf("canceled request lost: %+v %v", event, err)
	}
	if err := f.db.Exec("DROP TABLE request_logs").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.service.Record(context.Background(), &domain.RequestLog{}); err == nil {
		t.Fatal("failed write was hidden")
	}
	if err := f.db.AutoMigrate(&domain.RequestLog{}); err != nil {
		t.Fatal(err)
	}
	_, body := request(t, f, "GET", "/api/stats", "", 200, f.admin)
	if stats := decode[telemetry.Summary](t, body); stats.CollectionFailures != 1 {
		t.Fatalf("collection failure not reported: %+v", stats)
	}
}

func TestExportLimitAndFormulaEscaping(t *testing.T) {
	f := newFixture(t)
	event := domain.RequestLog{Timestamp: time.Now().UTC(), Hostname: "=1+1", Method: "GET", Path: "/", Action: "allowed", Status: 200}
	if err := f.service.Record(context.Background(), &event); err != nil {
		t.Fatal(err)
	}
	_, body := request(t, f, "GET", "/api/logs/export", "", 200, f.admin)
	rows, err := csv.NewReader(strings.NewReader(string(body))).ReadAll()
	if err != nil || rows[1][3] != "'=1+1" {
		t.Fatalf("unsafe CSV: %s err=%v", body, err)
	}
	if err := f.db.Exec(`WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<10000)
 INSERT INTO request_logs (timestamp,hostname,ip,method,path,action,rule_id,reason,status,error_category,duration_ms,request_bytes,response_bytes)
 SELECT ?, '', '', 'GET', '/', 'allowed', '', '', 200, '', 0, 0, 0 FROM n`, time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	request(t, f, "GET", "/api/logs/export", "", 413, f.admin)
}
