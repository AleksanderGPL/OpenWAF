package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"OpenWAF/internal/api"
	"OpenWAF/internal/auth"
	"OpenWAF/internal/database"
	"OpenWAF/internal/domain"
	"github.com/gofiber/fiber/v3"
)

func apiRequest(t *testing.T, app *fiber.App, method, path, body string, status int, cookies ...*http.Cookie) (*http.Response, []byte) {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		r.AddCookie(cookie)
	}
	response, err := app.Test(r, fiber.TestConfig{Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != status {
		t.Fatalf("%s %s: got %d want %d: %s (%v)", method, path, response.StatusCode, status, data, err)
	}
	return response, data
}

func TestServiceEndpointsAndPersistence(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "services.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	authService, err := auth.New(database.NewStore(db))
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{ErrorHandler: api.ErrorHandler})
	t.Cleanup(func() { app.Shutdown() })
	router := api.New(app).Group("/api")
	authHandler := auth.NewHandler(authService, false, nil)
	authHandler.Register(router)
	NewHandler(New(database.NewStore(db))).Register(router, authHandler.RequireAuth)
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		path := "/api/services"
		if method == "PUT" || method == "DELETE" {
			path += "/1"
		}
		apiRequest(t, app, method, path, "", 401)
	}
	apiRequest(t, app, "POST", "/api/auth/setup", `{"username":"admin","password":"test-password"}`, 201)
	response, _ := apiRequest(t, app, "POST", "/api/auth/sign-in", `{"username":"admin","password":"test-password"}`, 200)
	adminCookie := response.Cookies()[0]
	var admin domain.User
	if err := db.First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	user := domain.User{Username: "user", Name: "User", Role: "user", PasswordHash: admin.PasswordHash}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	response, _ = apiRequest(t, app, "POST", "/api/auth/sign-in", `{"username":"user","password":"test-password"}`, 200)
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		path := "/api/services"
		if method == "PUT" || method == "DELETE" {
			path += "/1"
		}
		apiRequest(t, app, method, path, "", 403, response.Cookies()[0])
	}
	_, data := apiRequest(t, app, "GET", "/api/services", "", 200, adminCookie)
	if string(data) != "[]" {
		t.Fatalf("empty list: %s", data)
	}
	for _, body := range []string{
		`{`, `null`, `{}`, `{"name":"x","hostname":"bad:80","upstreamUrl":"http://127.0.0.1"}`,
		`{"name":"x","hostname":"example.test","upstreamUrl":"ftp://127.0.0.1"}`,
		`{"name":"x","hostname":"example.test","upstreamUrl":"http://127.0.0.1:70000"}`,
		`{"name":"x","hostname":"example.test","upstreamUrl":"http://user:pass@127.0.0.1"}`,
		`{"name":"x","hostname":"example.test","upstreamUrl":"http://127.0.0.1/path"}`,
		`{"name":"x","hostname":"example.test","upstreamUrl":"http://127.0.0.1?x=y"}`,
	} {
		apiRequest(t, app, "POST", "/api/services", body, 400, adminCookie)
	}
	body := `{"name":" Example ","hostname":" EXAMPLE.TEST. ","upstreamUrl":"https://127.0.0.1:8443","skipTlsVerify":true}`
	_, data = apiRequest(t, app, "POST", "/api/services", body, 201, adminCookie)
	var created domain.Service
	if err := json.Unmarshal(data, &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Name != "Example" || created.Hostname != "example.test" || !created.Enabled || !created.SkipTLSVerify {
		t.Fatalf("unexpected service: %+v", created)
	}
	apiRequest(t, app, "POST", "/api/services", body, 409, adminCookie)
	path := fmt.Sprintf("/api/services/%d", created.ID)
	apiRequest(t, app, "GET", path, "", 200, adminCookie)
	apiRequest(t, app, "GET", "/api/services/0", "", 400, adminCookie)
	apiRequest(t, app, "GET", "/api/services/999", "", 404, adminCookie)
	body = `{"name":"Changed","hostname":"other.test","upstreamUrl":"http://127.0.0.1:9000","skipTlsVerify":false,"enabled":false}`
	_, data = apiRequest(t, app, "PUT", path, body, 200, adminCookie)
	var updated domain.Service
	if err := json.Unmarshal(data, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Enabled || updated.SkipTLSVerify || updated.Hostname != "other.test" || updated.CreatedAt != created.CreatedAt {
		t.Fatalf("update did not persist zero values: %+v", updated)
	}
	var persisted domain.Service
	if err := db.First(&persisted, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Enabled || persisted.SkipTLSVerify || persisted.UpstreamURL != updated.UpstreamURL || persisted.UpdatedAt.IsZero() {
		t.Fatalf("stored configuration differs: %+v", persisted)
	}
	var location struct{ File string }
	if err := db.Raw("PRAGMA database_list").Scan(&location).Error; err != nil {
		t.Fatal(err)
	}
	reopened, err := database.Open(location.File)
	if err != nil {
		t.Fatal(err)
	}
	reopenedSQL, _ := reopened.DB()
	defer reopenedSQL.Close()
	var reloaded domain.Service
	if err := reopened.First(&reloaded, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.UpstreamURL != updated.UpstreamURL || reloaded.Enabled || reloaded.SkipTLSVerify {
		t.Fatalf("configuration lost on reopening: %+v", reloaded)
	}
	apiRequest(t, app, "DELETE", path, "", 204, adminCookie)
	apiRequest(t, app, "GET", path, "", 404, adminCookie)
}
