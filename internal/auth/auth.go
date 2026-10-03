package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"mime"
	"net/http"
	"strings"
	"time"

	"OpenWAF/internal/api"
	"OpenWAF/internal/database"
	"github.com/alexedwards/argon2id"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"gorm.io/gorm"
)

const sessionLifetime = 30 * 24 * time.Hour

type SignInRequest struct {
	Username string `json:"username" required:"true" description:"Nonblank username, at most 255 bytes"`
	Password string `json:"password" required:"true" minLength:"1" format:"password" description:"At most 1024 bytes"`
}

type Service struct {
	db           *gorm.DB
	secureCookie bool
	dummyHash    string
}

func New(db *gorm.DB, secureCookie bool) (*Service, error) {
	hash, err := argon2id.CreateHash(hex.EncodeToString(randomBytes()), passwordParams)
	if err != nil {
		return nil, err
	}
	return &Service{db: db, secureCookie: secureCookie, dummyHash: hash}, nil
}

func randomBytes() []byte {
	b := make([]byte, 32)
	rand.Read(b)
	return b
}

func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (s *Service) Register(router *api.Router) {
	auth := router.Group("/auth", func(c fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		return c.Next()
	})
	auth.Handle(http.MethodGet, "/setup", api.Operation{ID: "getSetupStatus", Summary: "Check whether the first admin exists", Response: SetupStatus{}}, s.setupStatus)
	auth.Handle(http.MethodPost, "/setup", api.Operation{
		ID: "completeSetup", Summary: "Register the first admin", Request: SetupRequest{}, Response: database.User{},
		Status: 201, Errors: []int{400, 409, 415, 429},
	}, limiter.New(limiter.Config{
		Max: 10, Expiration: time.Minute,
		LimitReached: func(c fiber.Ctx) error { return fiber.ErrTooManyRequests },
	}), s.completeSetup)
	auth.Handle(http.MethodPost, "/sign-in", api.Operation{
		ID: "signIn", Summary: "Sign in", Description: "Sets an HttpOnly session cookie valid for 30 days.",
		Request: SignInRequest{}, Response: database.User{}, Errors: []int{400, 401, 415, 429},
	}, limiter.New(limiter.Config{
		Max: 10, Expiration: time.Minute,
		LimitReached: func(c fiber.Ctx) error { return fiber.ErrTooManyRequests },
	}), s.signIn)
	auth.Handle(http.MethodPost, "/sign-out", api.Operation{ID: "signOut", Summary: "Revoke the session and clear its cookie", Status: 204}, s.signOut)
	auth.Handle(http.MethodGet, "/", api.Operation{ID: "getCurrentUser", Summary: "Get the current user", Response: database.User{}, Session: true, Errors: []int{401}}, s.RequireAuth, func(c fiber.Ctx) error {
		return c.JSON(c.Locals("authUser"))
	})
}

func (s *Service) cookie(c fiber.Ctx, name, value string, expires time.Time) {
	maxAge := int(sessionLifetime.Seconds())
	if value == "" {
		maxAge = -1
		expires = time.Unix(1, 0)
	}
	c.Cookie(&fiber.Cookie{
		Name: name, Value: value, Path: "/", HTTPOnly: true,
		Secure: s.secureCookie, SameSite: "Strict", MaxAge: maxAge, Expires: expires,
	})
}

func readJSON(c fiber.Ctx, body any) error {
	mediaType, _, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "Expected application/json")
	}
	if err := c.Bind().JSON(body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid JSON body")
	}
	return nil
}

func (s *Service) signIn(c fiber.Ctx) error {
	var body SignInRequest
	if err := readJSON(c, &body); err != nil {
		return err
	}
	if strings.TrimSpace(body.Username) == "" || len(body.Username) > 255 || body.Password == "" || len(body.Password) > 1024 {
		return fiber.NewError(fiber.StatusBadRequest, "Username and password are required and must be within length limits")
	}
	var user database.User
	err := s.db.Where("username = ?", body.Username).Limit(1).Find(&user).Error
	if err != nil {
		return err
	}
	hash := s.dummyHash
	if user.ID != 0 {
		hash = user.PasswordHash
	}
	match, err := argon2id.ComparePasswordAndHash(body.Password, hash)
	if err != nil {
		return err
	}
	if !match || user.ID == 0 {
		return fiber.NewError(fiber.StatusUnauthorized, "Invalid username or password")
	}
	token := hex.EncodeToString(randomBytes())
	expires := time.Now().UTC().Add(sessionLifetime)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ? OR token_hash = ?", time.Now().UTC(), tokenHash(c.Cookies("session"))).Delete(&database.UserSession{}).Error; err != nil {
			return err
		}
		return tx.Create(&database.UserSession{UserID: user.ID, TokenHash: tokenHash(token), ExpiresAt: expires}).Error
	})
	if err != nil {
		return err
	}
	s.cookie(c, "session", token, expires)
	return c.JSON(user)
}

func (s *Service) signOut(c fiber.Ctx) error {
	if token := c.Cookies("session"); token != "" {
		if err := s.db.Where("token_hash = ?", tokenHash(token)).Delete(&database.UserSession{}).Error; err != nil {
			return err
		}
	}
	s.cookie(c, "session", "", time.Time{})
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Service) sessionUser(c fiber.Ctx) (database.User, error) {
	token := c.Cookies("session")
	if len(token) != 64 {
		return database.User{}, fiber.ErrUnauthorized
	}
	if _, err := hex.DecodeString(token); err != nil {
		return database.User{}, fiber.ErrUnauthorized
	}
	var session database.UserSession
	err := s.db.Preload("User").Where("token_hash = ? AND expires_at > ?", tokenHash(token), time.Now().UTC()).Limit(1).Find(&session).Error
	if err != nil {
		return database.User{}, err
	}
	if session.ID == 0 || session.User.ID == 0 {
		return database.User{}, fiber.ErrUnauthorized
	}
	return session.User, nil
}

func (s *Service) RequireAuth(c fiber.Ctx) error {
	user, err := s.sessionUser(c)
	if err != nil {
		return err
	}
	c.Locals("authUser", user)
	return c.Next()
}

func ErrorHandler(c fiber.Ctx, err error) error {
	code, message := fiber.StatusInternalServerError, "Internal server error"
	var fiberError *fiber.Error
	if errors.As(err, &fiberError) {
		code, message = fiberError.Code, fiberError.Message
	} else {
		log.Printf("request failed: %v", err)
	}
	return c.Status(code).JSON(api.ErrorResponse{Message: message})
}
