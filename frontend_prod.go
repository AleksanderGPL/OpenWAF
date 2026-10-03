//go:build !dev

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

const listenAddress = ":3000"
const defaultSecureCookies = true

func serveFrontend(app *fiber.App) {
	strippedFS, err := fs.Sub(gui, "frontend/.output/public")
	if err != nil {
		log.Fatal(err)
	}
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
}
