package main

import (
	"errors"
	"log"
	"os"
	"strconv"

	"OpenWAF/internal/auth"
	"OpenWAF/internal/database"

	"github.com/gofiber/fiber/v3"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	path := os.Getenv("DATABASE_PATH")
	if path == "" {
		path = "data/openwaf.db"
	}
	db, err := database.Open(path)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	secure := defaultSecureCookies
	if value, ok := os.LookupEnv("AUTH_COOKIE_SECURE"); ok {
		secure, err = strconv.ParseBool(value)
		if err != nil {
			return errors.New("AUTH_COOKIE_SECURE must be a boolean")
		}
	}
	authService, err := auth.New(db, secure)
	if err != nil {
		return err
	}
	app := fiber.New(fiber.Config{ErrorHandler: auth.ErrorHandler, BodyLimit: 16 * 1024})

	api := app.Group("/api")
	authService.Register(api)
	api.Get("/stats", authService.RequireAuth, func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "WAF Active", "blocks": 127})
	})
	api.Use(func(c fiber.Ctx) error { return fiber.ErrNotFound })

	serveFrontend(app)
	return app.Listen(listenAddress)
}
