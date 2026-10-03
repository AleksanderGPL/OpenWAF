package main

import (
	"OpenWAF/internal/api"
	"OpenWAF/internal/assistant"
	"OpenWAF/internal/auth"
	"OpenWAF/internal/services"
	"OpenWAF/internal/telemetry"
	"github.com/gofiber/fiber/v3"
)

func registerAPI(app *fiber.App, authHandler *auth.Handler, serviceHandler *services.Handler, telemetryHandler *telemetry.Handler, assistantHandlers ...*assistant.Handler) error {
	routes := api.New(app).Group("/api")
	authHandler.Register(routes)
	serviceHandler.Register(routes, authHandler.RequireAuth)
	telemetryHandler.Register(routes, authHandler.RequireAuth)
	for _, handler := range assistantHandlers {
		handler.Register(routes, authHandler.RequireAuth)
	}
	if err := registerDocs(routes); err != nil {
		return err
	}
	routes.Use(func(c fiber.Ctx) error { return fiber.ErrNotFound })
	return nil
}
