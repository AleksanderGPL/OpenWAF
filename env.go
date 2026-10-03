package main

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

// Load the working directory's .env without replacing exported variables.
func loadEnv() error {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.New("could not load .env; check its syntax and permissions")
	}
	return nil
}
