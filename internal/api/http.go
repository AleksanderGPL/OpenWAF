package api

import (
	"errors"
	"log"
	"mime"
	"net/http"

	"OpenWAF/internal/domain"
	"github.com/gofiber/fiber/v3"
)

func ReadJSON(c fiber.Ctx, body any) error {
	mediaType, _, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return fiber.NewError(http.StatusUnsupportedMediaType, "Expected application/json")
	}
	if err := c.Bind().JSON(body); err != nil {
		return fiber.NewError(http.StatusBadRequest, "Invalid JSON body")
	}
	return nil
}

func ErrorHandler(c fiber.Ctx, err error) error {
	code, message := http.StatusInternalServerError, "Internal server error"
	var fiberError *fiber.Error
	var validation domain.ValidationError
	switch {
	case errors.As(err, &fiberError):
		code, message = fiberError.Code, fiberError.Message
	case errors.As(err, &validation):
		code, message = http.StatusBadRequest, validation.Error()
	case errors.Is(err, domain.ErrUnauthorized), errors.Is(err, domain.ErrInvalidCredentials):
		code, message = http.StatusUnauthorized, err.Error()
	case errors.Is(err, domain.ErrSetupCompleted), errors.Is(err, domain.ErrUsernameTaken), errors.Is(err, domain.ErrHostnameTaken):
		code, message = http.StatusConflict, err.Error()
	case errors.Is(err, domain.ErrServiceNotFound), errors.Is(err, domain.ErrLogNotFound):
		code, message = http.StatusNotFound, err.Error()
	default:
		log.Printf("request failed: %v", err)
	}
	return c.Status(code).JSON(ErrorResponse{Message: message})
}
