package bootstrap

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config contains process configuration passed to concrete adapters and commands.
// Setup credentials are validated only when the setup command runs.
type Config struct {
	DevicePassword string
	AdminEmail     string
	AdminPassword  string
	CookieSecure   bool
}

// LoadConfig reads optional dotenv files relative to the process working directory.
// Process values, including empty values, take precedence over .env.local and .env.
func LoadConfig() (Config, error) {
	if err := loadEnv(); err != nil {
		return Config{}, err
	}
	return Config{
		DevicePassword: os.Getenv("SHELLY_DEVICE_PASSWORD"),
		AdminEmail:     os.Getenv("SHELLY_ADMIN_EMAIL"),
		AdminPassword:  os.Getenv("SHELLY_ADMIN_PASSWORD"),
		CookieSecure:   os.Getenv("SHELLY_COOKIE_SECURE") == "true",
	}, nil
}

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
