package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"OpenWAF/internal/domain"
	"github.com/alexedwards/argon2id"
)

const sessionLifetime = 30 * 24 * time.Hour

type Store interface {
	SetupCompleted(context.Context) (bool, error)
	CreateFirstAdmin(context.Context, *domain.User) error
	UserByUsername(context.Context, string) (domain.User, error)
	CreateSession(context.Context, *domain.UserSession, string) error
	DeleteSession(context.Context, string) error
	SessionUser(context.Context, string, time.Time) (domain.User, error)
}

type SignInRequest struct {
	Username string `json:"username" required:"true" description:"Nonblank username, at most 255 bytes"`
	Password string `json:"password" required:"true" minLength:"1" format:"password" description:"At most 1024 bytes"`
}

type Session struct {
	User      domain.User
	Token     string
	ExpiresAt time.Time
}

type Service struct {
	store     Store
	dummyHash string
}

func New(store Store) (*Service, error) {
	hash, err := argon2id.CreateHash(hex.EncodeToString(randomBytes()), passwordParams)
	if err != nil {
		return nil, err
	}
	return &Service{store: store, dummyHash: hash}, nil
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

func (s *Service) SignIn(ctx context.Context, body SignInRequest, previousToken string) (Session, error) {
	if strings.TrimSpace(body.Username) == "" || len(body.Username) > 255 || body.Password == "" || len(body.Password) > 1024 {
		return Session{}, domain.ValidationError("Username and password are required and must be within length limits")
	}
	user, err := s.store.UserByUsername(ctx, body.Username)
	if err != nil {
		return Session{}, err
	}
	hash := s.dummyHash
	if user.ID != 0 {
		hash = user.PasswordHash
	}
	match, err := argon2id.ComparePasswordAndHash(body.Password, hash)
	if err != nil {
		return Session{}, err
	}
	if !match || user.ID == 0 {
		return Session{}, domain.ErrInvalidCredentials
	}
	token := hex.EncodeToString(randomBytes())
	expires := time.Now().UTC().Add(sessionLifetime)
	err = s.store.CreateSession(ctx, &domain.UserSession{UserID: user.ID, TokenHash: tokenHash(token), ExpiresAt: expires}, tokenHash(previousToken))
	if err != nil {
		return Session{}, err
	}
	return Session{User: user, Token: token, ExpiresAt: expires}, nil
}

func (s *Service) SignOut(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(ctx, tokenHash(token))
}

func (s *Service) SessionUser(ctx context.Context, token string) (domain.User, error) {
	if len(token) != 64 {
		return domain.User{}, domain.ErrUnauthorized
	}
	if _, err := hex.DecodeString(token); err != nil {
		return domain.User{}, domain.ErrUnauthorized
	}
	return s.store.SessionUser(ctx, tokenHash(token), time.Now().UTC())
}
