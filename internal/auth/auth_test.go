package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OpenWAF/internal/api"
	"OpenWAF/internal/database"
	"OpenWAF/internal/domain"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

type fixture struct {
	app *fiber.App
	db  *gorm.DB
}

func setup(t *testing.T, secure bool) fixture {
	t.Helper()
	f := setupEmpty(t, secure)
	request(t, f.app, "POST", "/api/auth/setup", `{"username":"admin","password":"test-password","name":"Administrator"}`, 201)
	return f
}

func setupEmpty(t *testing.T, secure bool) fixture {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	return fixture{app: appForDatabase(t, db, secure), db: db}
}

func appForDatabase(t *testing.T, db *gorm.DB, secure bool) *fiber.App {
	t.Helper()
	service, err := New(database.NewStore(db))
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(service, secure)
	app := fiber.New(fiber.Config{ErrorHandler: api.ErrorHandler})
	handler.Register(api.New(app).Group("/api"))
	app.Get("/api/stats", handler.RequireAuth, func(c fiber.Ctx) error { return c.SendStatus(200) })
	t.Cleanup(func() { app.Shutdown() })
	return app
}

func request(t *testing.T, app *fiber.App, method, path, body string, status int, cookies ...*http.Cookie) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	response, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != status {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, response.StatusCode, status, data)
	}
	if strings.Contains(string(data), "PasswordHash") || strings.Contains(string(data), "$argon2") || strings.Contains(string(data), "passwordHash") {
		t.Fatalf("response exposed a password hash: %s", data)
	}
	return response, string(data)
}

func cookie(t *testing.T, response *http.Response, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("missing %s cookie", name)
	return nil
}

func login(t *testing.T, app *fiber.App) *http.Cookie {
	t.Helper()
	response, _ := request(t, app, "POST", "/api/auth/sign-in", `{"username":"admin","password":"test-password"}`, 200)
	return cookie(t, response, "session")
}

func TestSessionLifecycle(t *testing.T) {
	f := setup(t, true)
	request(t, f.app, "GET", "/api/auth", "", 401)
	request(t, f.app, "GET", "/api/stats", "", 401)
	_, wrong := request(t, f.app, "POST", "/api/auth/sign-in", `{"username":"admin","password":"wrong"}`, 401)
	_, missing := request(t, f.app, "POST", "/api/auth/sign-in", `{"username":"missing","password":"wrong"}`, 401)
	if wrong != missing {
		t.Fatal("failed login reveals whether the username exists")
	}
	request(t, f.app, "POST", "/api/auth/sign-in", `{"username":"admin"}`, 400)
	request(t, f.app, "POST", "/api/auth/sign-in", "{", 400)
	session := login(t, f.app)
	if len(session.Value) != 64 || !session.HttpOnly || !session.Secure || session.SameSite != http.SameSiteStrictMode || session.Path != "/" || session.MaxAge != 2592000 {
		t.Fatalf("unexpected session cookie: %+v", session)
	}
	var stored domain.UserSession
	if err := f.db.First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TokenHash == session.Value || stored.TokenHash != tokenHash(session.Value) {
		t.Fatal("session token was not hashed in storage")
	}
	if time.Until(stored.ExpiresAt) < sessionLifetime-time.Minute {
		t.Fatal("incorrect session expiry")
	}
	response, data := request(t, f.app, "GET", "/api/auth", "", 200, session)
	if response.Header.Get("Cache-Control") != "no-store" || !strings.Contains(data, `"username":"admin"`) {
		t.Fatalf("unexpected auth response: %s", data)
	}
	request(t, f.app, "GET", "/api/stats", "", 200, session)
	response, _ = request(t, f.app, "POST", "/api/auth/sign-out", "", 204, session)
	cleared := cookie(t, response, "session")
	if cleared.MaxAge != -1 || !cleared.HttpOnly || !cleared.Secure || cleared.Path != "/" {
		t.Fatalf("incorrect cleared cookie: %+v", cleared)
	}
	request(t, f.app, "GET", "/api/auth", "", 401, session)
	request(t, f.app, "POST", "/api/auth/sign-out", "", 204)
	session = login(t, f.app)
	if err := f.db.Model(&domain.UserSession{}).Where("token_hash = ?", tokenHash(session.Value)).Update("expires_at", time.Now().UTC().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	request(t, f.app, "GET", "/api/auth", "", 401, session)
}

func TestSessionReplacementAndUserDeletion(t *testing.T) {
	f := setup(t, false)
	var admin domain.User
	f.db.First(&admin)
	user := domain.User{Username: "user", Name: "User", PasswordHash: admin.PasswordHash, Role: "user"}
	if err := f.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	session := login(t, f.app)
	if session.Secure {
		t.Fatal("development cookie must work over HTTP")
	}
	response, _ := request(t, f.app, "POST", "/api/auth/sign-in", `{"username":"user","password":"test-password"}`, 200)
	userSession := cookie(t, response, "session")
	_, data := request(t, f.app, "GET", "/api/auth", "", 200, userSession)
	if !strings.Contains(data, `"username":"user"`) {
		t.Fatalf("incorrect session identity: %s", data)
	}
	response, _ = request(t, f.app, "POST", "/api/auth/sign-in", `{"username":"admin","password":"test-password"}`, 200, session)
	request(t, f.app, "GET", "/api/auth", "", 401, session)
	request(t, f.app, "GET", "/api/auth", "", 200, cookie(t, response, "session"))
	if err := f.db.Delete(&user).Error; err != nil {
		t.Fatal(err)
	}
	request(t, f.app, "GET", "/api/auth", "", 401, userSession)
	var count int64
	f.db.Model(&domain.UserSession{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 0 {
		t.Fatal("user deletion did not cascade to sessions")
	}
}

func TestSetupPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "auth.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	app := appForDatabase(t, db, false)
	_, status := request(t, app, "GET", "/api/auth/setup", "", 200)
	if status != `{"completed":false}` {
		t.Fatalf("unexpected setup status: %s", status)
	}
	request(t, app, "POST", "/api/auth/setup", `{"username":"admin","password":"short"}`, 400)
	request(t, app, "POST", "/api/auth/setup", `{"username":"admin","password":"test-password"}`, 201)
	session := login(t, app)
	var original domain.User
	db.First(&original)
	storedSession := domain.UserSession{UserID: original.ID, TokenHash: tokenHash("persistent-token"), ExpiresAt: time.Now().UTC().Add(sessionLifetime)}
	if err := db.Create(&storedSession).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.Close()
	db, err = database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ = db.DB()
	defer sqlDB.Close()
	app = appForDatabase(t, db, false)
	_, status = request(t, app, "GET", "/api/auth/setup", "", 200)
	if status != `{"completed":true}` {
		t.Fatalf("setup status did not persist: %s", status)
	}
	request(t, app, "POST", "/api/auth/setup", `{"username":"replacement","password":"other-password"}`, 409)
	request(t, app, "GET", "/api/auth", "", 200, session)
	var users []domain.User
	db.Find(&users)
	if len(users) != 1 || users[0].Username != "admin" || users[0].PasswordHash != original.PasswordHash {
		t.Fatal("setup replaced an existing account or database did not persist")
	}
	var restoredSession domain.UserSession
	if err := db.Preload("User").First(&restoredSession, storedSession.ID).Error; err != nil {
		t.Fatal(err)
	}
	if restoredSession.TokenHash != storedSession.TokenHash || restoredSession.User.Username != "admin" {
		t.Fatal("session did not persist across database reopen")
	}
}

func TestSetupValidation(t *testing.T) {
	f := setupEmpty(t, false)
	for _, body := range []string{
		`{`, `{}`, `{"username":"  ","password":"test-password"}`,
		`{"username":"admin","password":"short"}`,
		`{"username":"admin","password":"test-password","name":123}`,
	} {
		request(t, f.app, "POST", "/api/auth/setup", body, 400)
	}
	_, status := request(t, f.app, "GET", "/api/auth/setup", "", 200)
	if status != `{"completed":false}` {
		t.Fatalf("invalid requests changed setup status: %s", status)
	}
	response, data := request(t, f.app, "POST", "/api/auth/setup", `{"username":" admin ","password":"test-password","role":"user"}`, 201)
	var user domain.User
	if err := json.Unmarshal([]byte(data), &user); err != nil {
		t.Fatal(err)
	}
	if user.Username != "admin" || user.Name != "admin" || user.Role != "admin" {
		t.Fatalf("unexpected first admin: %s", data)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("setup response must not be cached")
	}
	request(t, f.app, "POST", "/api/auth/setup", `{"username":"second","password":"test-password"}`, 409)
	login(t, f.app)
}

func TestConcurrentSetup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.db")
	var apps []*fiber.App
	for range 2 {
		db, err := database.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, _ := db.DB()
		t.Cleanup(func() { sqlDB.Close() })
		apps = append(apps, appForDatabase(t, db, false))
	}
	type result struct {
		status int
		err    error
	}
	start := make(chan struct{})
	results := make(chan result, len(apps))
	for _, app := range apps {
		go func() {
			<-start
			req := httptest.NewRequest("POST", "/api/auth/setup", strings.NewReader(`{"username":"admin","password":"test-password"}`))
			req.Header.Set("Content-Type", "application/json")
			response, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
			if err != nil {
				results <- result{err: err}
				return
			}
			response.Body.Close()
			results <- result{status: response.StatusCode}
		}()
	}
	close(start)
	statuses := map[int]int{}
	for range apps {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		statuses[result.status]++
	}
	if statuses[201] != 1 || statuses[409] != 1 {
		t.Fatalf("expected one successful setup and one conflict: %v", statuses)
	}
	for _, app := range apps {
		_, status := request(t, app, "GET", "/api/auth/setup", "", 200)
		if status != `{"completed":true}` {
			t.Fatalf("unexpected setup status: %s", status)
		}
	}
}

func TestLoginRateLimit(t *testing.T) {
	f := setup(t, false)
	for range 10 {
		request(t, f.app, "POST", "/api/auth/sign-in", `{}`, 400)
	}
	request(t, f.app, "POST", "/api/auth/sign-in", `{}`, 429)
}
