package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func loadEnv() error {
	// Load preserves existing variables, so local values take precedence over defaults.
	for _, path := range []string{".env.local", ".env"} {
		if err := godotenv.Load(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			// Parser errors can include file contents, which may contain credentials.
			return fmt.Errorf("could not load %s; check its syntax and permissions", path)
		}
	}
	return nil
}
