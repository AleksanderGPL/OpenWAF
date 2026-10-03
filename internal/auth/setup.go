package auth

import (
	"context"
	"strings"
	"unicode/utf8"

	"OpenWAF/internal/domain"
	"github.com/alexedwards/argon2id"
)

var passwordParams = &argon2id.Params{
	Memory: 64 * 1024, Iterations: 3, Parallelism: 4, SaltLength: 16, KeyLength: 32,
}

type SetupRequest struct {
	Username string `json:"username" required:"true" description:"Trimmed username, 1–255 bytes"`
	Password string `json:"password" required:"true" minLength:"8" format:"password" description:"At least 8 characters and at most 1024 bytes"`
	Name     string `json:"name" description:"At most 255 bytes; defaults to username when empty"`
}

type SetupStatus struct {
	Completed bool `json:"completed" required:"true"`
}

func (s *Service) SetupCompleted(ctx context.Context) (bool, error) {
	return s.store.SetupCompleted(ctx)
}

func (s *Service) CompleteSetup(ctx context.Context, body SetupRequest) (domain.User, error) {
	body.Username = strings.TrimSpace(body.Username)
	body.Name = strings.TrimSpace(body.Name)
	if body.Username == "" || len(body.Username) > 255 || utf8.RuneCountInString(body.Password) < 8 || len(body.Password) > 1024 || len(body.Name) > 255 {
		return domain.User{}, domain.ValidationError("Username must be 1–255 bytes, password at least 8 characters and at most 1024 bytes, and name at most 255 bytes")
	}
	if body.Name == "" {
		body.Name = body.Username
	}
	hash, err := argon2id.CreateHash(body.Password, passwordParams)
	if err != nil {
		return domain.User{}, err
	}
	user := domain.User{Username: body.Username, Name: body.Name, PasswordHash: hash, Role: "admin"}

	if err := s.store.CreateFirstAdmin(ctx, &user); err != nil {
		return domain.User{}, err
	}
	return user, nil
}
