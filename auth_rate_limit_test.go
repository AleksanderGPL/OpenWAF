package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthRateLimitBehindProxy(t *testing.T) {
	for _, path := range []string{"/api/auth/sign-in", "/api/auth/setup"} {
		t.Run(path, func(t *testing.T) {
			t.Setenv("TRUSTED_PROXIES", "0.0.0.0")
			app := testAPI(t)
			send := func(forwarded string, want int) {
				t.Helper()
				request := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Forwarded-For", forwarded)
				response, err := app.Test(request)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				if response.StatusCode != want {
					t.Fatalf("forwarded %q: got %d, want %d", forwarded, response.StatusCode, want)
				}
			}
			for range 10 {
				send("198.51.100.1", 400)
			}
			send("203.0.113.1, 198.51.100.1", 429)
			send("198.51.100.2", 400)
		})
	}
}

func TestAuthRateLimitIgnoresUntrustedHeaders(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1,::1")
	app := testAPI(t)
	for i := 0; i < 11; i++ {
		request := httptest.NewRequest("POST", "/api/auth/sign-in", strings.NewReader(`{}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Forwarded-For", "198.51.100.1")
		want := 400
		if i == 10 {
			request.Header.Set("X-Forwarded-For", "198.51.100.2")
			want = 429
		}
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("request %d: got %d, want %d", i, response.StatusCode, want)
		}
	}
}
