package auth

import (
	"strings"
	"unicode/utf8"

	"OpenWAF/internal/database"
	"github.com/alexedwards/argon2id"
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
)

var passwordParams = &argon2id.Params{
	Memory: 64 * 1024, Iterations: 3, Parallelism: 4, SaltLength: 16, KeyLength: 32,
}

func setupCompleted(db *gorm.DB) (bool, error) {
	var user database.User
	err := db.Select("id").Where("role = ?", "admin").Limit(1).Find(&user).Error
	return user.ID != 0, err
}

func (s *Service) setupStatus(c fiber.Ctx) error {
	completed, err := setupCompleted(s.db)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"completed": completed})
}

func (s *Service) completeSetup(c fiber.Ctx) error {
	completed, err := setupCompleted(s.db)
	if err != nil {
		return err
	}
	if completed {
		return fiber.NewError(fiber.StatusConflict, "Setup has already been completed")
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := readJSON(c, &body); err != nil {
		return err
	}
	body.Username = strings.TrimSpace(body.Username)
	body.Name = strings.TrimSpace(body.Name)
	if body.Username == "" || len(body.Username) > 255 || utf8.RuneCountInString(body.Password) < 8 || len(body.Password) > 1024 || len(body.Name) > 255 {
		return fiber.NewError(fiber.StatusBadRequest, "Username must be 1–255 bytes, password at least 8 characters and at most 1024 bytes, and name at most 255 bytes")
	}
	if body.Name == "" {
		body.Name = body.Username
	}
	hash, err := argon2id.CreateHash(body.Password, passwordParams)
	if err != nil {
		return err
	}
	user := database.User{Username: body.Username, Name: body.Name, PasswordHash: hash, Role: "admin"}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		completed, err := setupCompleted(tx)
		if err != nil {
			return err
		}
		if completed {
			return fiber.NewError(fiber.StatusConflict, "Setup has already been completed")
		}
		return tx.Create(&user).Error
	})
	if err != nil {
		if err == gorm.ErrDuplicatedKey {
			return fiber.NewError(fiber.StatusConflict, "Username is already taken")
		}
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(user)
}
