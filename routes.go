package main

import (
	"net/http"

	"OpenWAF/internal/api"
	"OpenWAF/internal/auth"
	"OpenWAF/internal/services"
	"github.com/gofiber/fiber/v3"
)

type StatsResponse struct {
	Status string `json:"status" required:"true"`
	Blocks int    `json:"blocks" required:"true"`
}

func registerAPI(app *fiber.App, authHandler *auth.Handler, serviceHandler *services.Handler) error {
	routes := api.New(app).Group("/api")
	authHandler.Register(routes)
	serviceHandler.Register(routes, authHandler.RequireAuth)
	routes.Handle(http.MethodGet, "/stats", api.Operation{
		ID: "getStats", Summary: "Get placeholder WAF statistics", Response: StatsResponse{}, Session: true, Errors: []int{401},
	}, authHandler.RequireAuth, func(c fiber.Ctx) error {
		return c.JSON(StatsResponse{Status: "WAF Active", Blocks: 127})
	})
	if err := registerDocs(routes); err != nil {
		return err
	}
	routes.Use(func(c fiber.Ctx) error { return fiber.ErrNotFound })
	return nil
}
