package auth

import (
	"net/http"
	"time"

	"OpenWAF/internal/api"
	"OpenWAF/internal/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
)

type Handler struct {
	service      *Service
	secureCookie bool
	rateLimitKey func(fiber.Ctx) string
}

func NewHandler(service *Service, secureCookie bool, rateLimitKey func(fiber.Ctx) string) *Handler {
	return &Handler{service: service, secureCookie: secureCookie, rateLimitKey: rateLimitKey}
}

func (h *Handler) authLimiter() fiber.Handler {
	return limiter.New(limiter.Config{Max: 10, Expiration: time.Minute, KeyGenerator: h.rateLimitKey,
		LimitReached: func(c fiber.Ctx) error { return fiber.ErrTooManyRequests },
	})
}

func (h *Handler) Register(router *api.Router) {
	auth := router.Group("/auth", func(c fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		return c.Next()
	})
	auth.Handle(http.MethodGet, "/setup", api.Operation{ID: "getSetupStatus", Summary: "Check whether the first admin exists", Response: SetupStatus{}}, h.setupStatus)
	auth.Handle(http.MethodPost, "/setup", api.Operation{
		ID: "completeSetup", Summary: "Register the first admin", Request: SetupRequest{}, Response: domain.User{},
		Status: 201, Errors: []int{400, 409, 415, 429},
	}, h.authLimiter(), h.completeSetup)
	auth.Handle(http.MethodPost, "/sign-in", api.Operation{
		ID: "signIn", Summary: "Sign in", Description: "Sets an HttpOnly session cookie valid for 30 days.",
		Request: SignInRequest{}, Response: domain.User{}, Errors: []int{400, 401, 415, 429},
	}, h.authLimiter(), h.signIn)
	auth.Handle(http.MethodPost, "/sign-out", api.Operation{ID: "signOut", Summary: "Revoke the session and clear its cookie", Status: 204}, h.signOut)
	auth.Handle(http.MethodGet, "/", api.Operation{ID: "getCurrentUser", Summary: "Get the current user", Response: domain.User{}, Session: true, Errors: []int{401}}, h.RequireAuth, func(c fiber.Ctx) error {
		return c.JSON(c.Locals("authUser"))
	})
}

func (h *Handler) cookie(c fiber.Ctx, value string, expires time.Time) {
	maxAge := int(sessionLifetime.Seconds())
	if value == "" {
		maxAge = -1
		expires = time.Unix(1, 0)
	}
	c.Cookie(&fiber.Cookie{
		Name: "session", Value: value, Path: "/", HTTPOnly: true,
		Secure: h.secureCookie, SameSite: "Strict", MaxAge: maxAge, Expires: expires,
	})
}

func (h *Handler) setupStatus(c fiber.Ctx) error {
	completed, err := h.service.SetupCompleted(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(SetupStatus{Completed: completed})
}

func (h *Handler) completeSetup(c fiber.Ctx) error {
	completed, err := h.service.SetupCompleted(c.Context())
	if err != nil {
		return err
	}
	if completed {
		return domain.ErrSetupCompleted
	}
	var body SetupRequest
	if err := api.ReadJSON(c, &body); err != nil {
		return err
	}
	user, err := h.service.CompleteSetup(c.Context(), body)
	if err != nil {
		return err
	}
	return c.Status(http.StatusCreated).JSON(user)
}

func (h *Handler) signIn(c fiber.Ctx) error {
	var body SignInRequest
	if err := api.ReadJSON(c, &body); err != nil {
		return err
	}
	session, err := h.service.SignIn(c.Context(), body, c.Cookies("session"))
	if err != nil {
		return err
	}
	h.cookie(c, session.Token, session.ExpiresAt)
	return c.JSON(session.User)
}

func (h *Handler) signOut(c fiber.Ctx) error {
	if err := h.service.SignOut(c.Context(), c.Cookies("session")); err != nil {
		return err
	}
	h.cookie(c, "", time.Time{})
	return c.SendStatus(http.StatusNoContent)
}

func (h *Handler) RequireAuth(c fiber.Ctx) error {
	user, err := h.service.SessionUser(c.Context(), c.Cookies("session"))
	if err != nil {
		return err
	}
	c.Locals("authUser", user)
	return c.Next()
}
