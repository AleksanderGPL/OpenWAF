package main

import (
	"log"

	"github.com/gofiber/fiber/v3"
)

func main() {
	app := fiber.New()

	api := app.Group("/api")
	api.Get("/stats", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "WAF Active", "blocks": 127})
	})

	serveFrontend(app)
	log.Fatal(app.Listen(listenAddress))
}
