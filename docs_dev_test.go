//go:build dev

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDevelopmentOpenAPI(t *testing.T) {
	app := testAPI(t)
	response, body := getAPI(t, app, "/api/openapi.json", 200)
	if !strings.Contains(response.Header.Get("Content-Type"), "application/json") {
		t.Fatal("schema is not served as JSON")
	}
	var spec struct {
		OpenAPI string `json:"openapi"`
		Paths   map[string]map[string]struct {
			OperationID string                       `json:"operationId"`
			RequestBody json.RawMessage              `json:"requestBody"`
			Responses   map[string]json.RawMessage   `json:"responses"`
			Parameters  []map[string]json.RawMessage `json:"parameters"`
			Security    []map[string][]string        `json:"security"`
		} `json:"paths"`
		Components struct {
			Schemas         map[string]json.RawMessage `json:"schemas"`
			SecuritySchemes map[string]struct {
				Type string `json:"type"`
				In   string `json:"in"`
				Name string `json:"name"`
			} `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.OpenAPI != "3.0.3" || len(spec.Paths) != 15 {
		t.Fatalf("unexpected schema: %s paths=%d", spec.OpenAPI, len(spec.Paths))
	}
	expected := map[string][]string{
		"/api/auth/setup": {"get", "post"}, "/api/auth/sign-in": {"post"},
		"/api/auth/sign-out": {"post"}, "/api/auth": {"get"},
		"/api/services": {"get", "post"}, "/api/services/{id}": {"get", "put", "delete"}, "/api/stats": {"get"},
		"/api/stats/traffic": {"get"}, "/api/stats/threats": {"get"}, "/api/stats/blocked-sources": {"get"},
		"/api/stats/countries": {"get"},
		"/api/logs":            {"get"}, "/api/logs/export": {"get"}, "/api/logs/{id}": {"get"}, "/api/settings": {"get", "put"},
	}
	ids := make(map[string]bool)
	for path, methods := range expected {
		for _, method := range methods {
			op, ok := spec.Paths[path][method]
			if !ok || op.OperationID == "" || ids[op.OperationID] || op.Responses["500"] == nil {
				t.Fatalf("missing or invalid operation: %s %s", method, path)
			}
			ids[op.OperationID] = true
		}
	}
	if len(spec.Paths["/api/auth/setup"]["post"].Security) != 0 || len(spec.Paths["/api/services"]["post"].Security) != 1 {
		t.Fatal("incorrect authentication requirements")
	}
	security := spec.Components.SecuritySchemes["session"]
	if security.Type != "apiKey" || security.In != "cookie" || security.Name != "session" {
		t.Fatalf("incorrect cookie authentication scheme: %+v", security)
	}
	update := spec.Paths["/api/services/{id}"]["put"]
	if len(update.Parameters) != 1 || string(update.Parameters[0]["name"]) != `"id"` || string(update.Parameters[0]["required"]) != "true" || !strings.Contains(string(update.RequestBody), `"required": true`) {
		t.Fatalf("missing service input or ID: %+v", update)
	}
	if strings.Contains(string(spec.Paths["/api/services/{id}"]["delete"].Responses["204"]), `"content"`) {
		t.Fatal("204 response incorrectly includes a body")
	}

	for path, names := range map[string][]string{
		"/api/stats":           {"range", "from", "to", "serviceId"},
		"/api/stats/countries": {"range", "from", "to", "serviceId", "mode"},
		"/api/logs":            {"range", "from", "to", "serviceId", "action", "ip", "method", "status", "ruleId", "search", "page", "limit"},
	} {
		parameters := make(map[string]bool)
		for _, parameter := range spec.Paths[path]["get"].Parameters {
			var name string
			if err := json.Unmarshal(parameter["name"], &name); err != nil {
				t.Fatal(err)
			}
			if string(parameter["in"]) != `"query"` {
				t.Fatalf("incorrect parameter location: %s %s", path, name)
			}
			parameters[name] = true
		}
		for _, name := range names {
			if !parameters[name] {
				t.Errorf("%s missing parameter %s", path, name)
			}
		}
	}
	if !strings.Contains(string(spec.Paths["/api/logs/export"]["get"].Responses["200"]), `"text/csv"`) {
		t.Fatal("CSV response missing from schema")
	}
	for _, path := range []string{"/api/stats", "/api/stats/traffic", "/api/stats/threats", "/api/stats/blocked-sources", "/api/stats/countries", "/api/logs", "/api/logs/export", "/api/logs/{id}", "/api/settings"} {
		if len(spec.Paths[path]["get"].Security) != 1 {
			t.Fatalf("missing session security for %s", path)
		}
	}

	for name, schema := range spec.Components.Schemas {
		if strings.Contains(strings.ToLower(string(schema)), "passwordhash") {
			t.Fatalf("%s exposes password hashes", name)
		}
	}
	for _, field := range []string{"username", "password", "completed", "upstreamUrl", "skipTlsVerify", "enabled", "logRetentionDays", "durationMs", "requestBytes", "responseBytes", "collectionFailures"} {
		if !strings.Contains(string(body), `"`+field+`"`) {
			t.Errorf("schema missing %s", field)
		}
	}
}

func TestDevelopmentSwagger(t *testing.T) {
	app := testAPI(t)
	getAPI(t, app, "/api/swagger", 200)
	_, body := getAPI(t, app, "/api/swagger/", 200)
	if !strings.Contains(string(body), "/api/openapi.json") || !strings.Contains(string(body), "SwaggerUIBundle") {
		t.Fatal("Swagger UI does not reference the generated schema")
	}
	getAPI(t, app, "/api/swagger/swagger-ui.css", 200)
	getAPI(t, app, "/api/swagger/swagger-ui-bundle.js", 200)
}
