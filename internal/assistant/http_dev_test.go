//go:build dev

package assistant

import (
	"OpenWAF/internal/api"
	"encoding/json"
	"github.com/gofiber/fiber/v3"
	"testing"
)

func TestAssistantOpenAPI(t *testing.T) {
	app := fiber.New()
	router := api.New(app).Group("/api")
	NewHandler(&Service{}).Register(router, func(c fiber.Ctx) error { return c.Next() })
	document, err := router.Schema()
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(document, &spec); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/assistant/status", "/api/assistant/conversations", "/api/assistant/conversations/{id}/messages", "/api/assistant/conversations/{id}/cancel", "/api/assistant/conversations/{id}"} {
		if _, ok := spec.Paths[path]; !ok {
			t.Fatalf("missing documented path %s", path)
		}
	}
}
