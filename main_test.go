package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"OpenWAF/internal/auth"
	"OpenWAF/internal/database"
	"OpenWAF/internal/proxy"
	"github.com/gofiber/fiber/v3"
)

func testAPI(t *testing.T) *fiber.App {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	authService, err := auth.New(db, false)
	if err != nil {
		t.Fatal(err)
	}
	proxyService := proxy.New(db)
	t.Cleanup(proxyService.Close)
	app := fiber.New(fiber.Config{ErrorHandler: auth.ErrorHandler})
	t.Cleanup(func() { app.Shutdown() })
	if err := registerAPI(app, authService, proxyService); err != nil {
		t.Fatal(err)
	}
	serveFrontend(app)
	return app
}

func getAPI(t *testing.T, app *fiber.App, path string, status int) (*http.Response, []byte) {
	t.Helper()
	response, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != status {
		t.Fatalf("GET %s: got %d want %d: %s (%v)", path, response.StatusCode, status, body, err)
	}
	return response, body
}
