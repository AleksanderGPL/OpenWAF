package domain

import "errors"

var (
	ErrUnauthorized       = errors.New("Unauthorized")
	ErrInvalidCredentials = errors.New("Invalid username or password")
	ErrSetupCompleted     = errors.New("Setup has already been completed")
	ErrUsernameTaken      = errors.New("Username is already taken")
	ErrHostnameTaken      = errors.New("A service with this hostname already exists")
	ErrLogNotFound        = errors.New("Request log not found")
	ErrServiceNotFound    = errors.New("Not Found")
)

type ValidationError string

func (e ValidationError) Error() string { return string(e) }
