//go:build dev

package main

import (
	"OpenWAF/internal/api"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/swaggest/swgui/v5emb"
)

func registerDocs(router *api.Router) error {
	if err := router.Publish("/openapi.json"); err != nil {
		return err
	}
	handler := adaptor.HTTPHandler(v5emb.New("OpenWAF API", "/api/openapi.json", "/api/swagger/"))
	router.Get("/swagger", handler)
	router.Get("/swagger/*", handler)
	return nil
}
