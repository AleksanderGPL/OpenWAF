package api

import (
	"OpenWAF/internal/clientip"
	"github.com/gofiber/fiber/v3"
)

func ClientIPKey(proxies []string) (func(fiber.Ctx) string, error) {
	resolver, err := clientip.New(proxies)
	if err != nil {
		return nil, err
	}
	return func(c fiber.Ctx) string {
		var headers []string
		for _, value := range c.Request().Header.PeekAll("X-Forwarded-For") {
			headers = append(headers, string(value))
		}
		return resolver.IP(c.RequestCtx().RemoteIP().String(), headers)
	}, nil
}
