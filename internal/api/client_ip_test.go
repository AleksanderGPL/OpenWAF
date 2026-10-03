package api

import (
	"net"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestClientIPKey(t *testing.T) {
	tests := []struct {
		name                  string
		proxies               []string
		peer, forwarded, want string
	}{
		{"trusted local proxy", []string{"127.0.0.1"}, "127.0.0.1", "198.51.100.1", "198.51.100.1"},
		{"untrusted peer", []string{"127.0.0.1"}, "198.51.100.1", "203.0.113.1", "198.51.100.1"},
		{"trust disabled", nil, "127.0.0.1", "203.0.113.1", "127.0.0.1"},
		{"prepended spoof", []string{"127.0.0.1"}, "127.0.0.1", "203.0.113.1, 198.51.100.1", "198.51.100.1"},
		{"duplicate header lines", []string{"127.0.0.1"}, "127.0.0.1", "203.0.113.1\n198.51.100.1", "198.51.100.1"},
		{"multiple trusted proxies", []string{"127.0.0.1", "10.1.0.0/24"}, "127.0.0.1", "203.0.113.1, 198.51.100.1, 10.1.0.2", "198.51.100.1"},
		{"missing header", []string{"127.0.0.1"}, "127.0.0.1", "", "127.0.0.1"},
		{"malformed header", []string{"127.0.0.1"}, "127.0.0.1", "198.51.100.1, invalid", "127.0.0.1"},
		{"empty last hop", []string{"127.0.0.1"}, "127.0.0.1", "198.51.100.1,", "127.0.0.1"},
		{"IPv6 proxy and client", []string{"::1"}, "::1", "2001:0db8:0:0:0:0:0:1", "2001:db8::1"},
		{"mapped IPv4 client", []string{"127.0.0.1"}, "127.0.0.1", "::ffff:198.51.100.1", "198.51.100.1"},
		{"zoned IPv6 client", []string{"::1"}, "::1", "fe80::1%arbitrary", "::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := ClientIPKey(tt.proxies)
			if err != nil {
				t.Fatal(err)
			}
			app := fiber.New()
			t.Cleanup(func() { app.Shutdown() })
			app.Get("/", func(c fiber.Ctx) error {
				c.RequestCtx().SetRemoteAddr(&net.TCPAddr{IP: net.ParseIP(tt.peer), Port: 12345})
				if got := key(c); got != tt.want {
					t.Errorf("got %q, want %q", got, tt.want)
				}
				return c.SendStatus(204)
			})
			request := httptest.NewRequest("GET", "/", nil)
			for _, value := range strings.Split(tt.forwarded, "\n") {
				request.Header.Add("X-Forwarded-For", value)
			}
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
		})
	}
}

func TestClientIPKeyRejectsInvalidProxies(t *testing.T) {
	for _, value := range []string{"", "localhost", "127.0.0.1:8080", "10.1.0.0/99", "fe80::1%eth0"} {
		if _, err := ClientIPKey([]string{value}); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}
