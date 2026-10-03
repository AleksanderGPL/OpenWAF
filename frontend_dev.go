//go:build dev

package main

import "github.com/gofiber/fiber/v3"

const listenAddress = "127.0.0.1:3001"
const defaultSecureCookies = false

func serveFrontend(app *fiber.App) {}
