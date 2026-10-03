//go:build !dev

package main

import "testing"

func TestProductionDoesNotExposeDocumentation(t *testing.T) {
	app := testAPI(t)
	for _, path := range []string{"/api/openapi.json", "/api/swagger", "/api/swagger/", "/api/swagger/swagger-ui-bundle.js"} {
		getAPI(t, app, path, 404)
	}
}
