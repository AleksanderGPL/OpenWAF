package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

//go:embed frontend/.output/public/*
var gui embed.FS

func main() {
	app := fiber.New()

	strippedFS, err := fs.Sub(gui, "frontend/.output/public")
	if err != nil {
		log.Fatal(err)
	}

	api := app.Group("/api")
	api.Get("/stats", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "WAF Active", "blocks": 127})
	})

	app.Use("/", static.New("", static.Config{
		FS: strippedFS,
	}))

	app.Use(func(c fiber.Ctx) error {
		index, err := fs.ReadFile(strippedFS, "index.html")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("index.html not found")
		}
		c.Type("html")
		return c.Send(index)
	})

	log.Fatal(app.Listen(":3000"))
}
