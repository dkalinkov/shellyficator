package bootstrap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config contains process configuration passed to concrete adapters and commands.
// Setup credentials are validated only when the setup command runs.
type Config struct {
	DevicePassword   string
	AdminEmail       string
	AdminPassword    string
	CookieSecure     bool
	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string
	HomeAssistant    bool
}

// LoadConfig reads the optional SHELLY_OPTIONS_FILE, then optional dotenv files
// relative to the process working directory. Process values, including empty
// values, take precedence over the options file, then .env.local, then .env.
func LoadConfig() (Config, error) {
	if path := os.Getenv("SHELLY_OPTIONS_FILE"); path != "" {
		if err := loadOptions(path); err != nil {
			return Config{}, err
		}
	}
	if err := loadEnv(); err != nil {
		return Config{}, err
	}
	return Config{
		DevicePassword:   os.Getenv("SHELLY_DEVICE_PASSWORD"),
		AdminEmail:       os.Getenv("SHELLY_ADMIN_EMAIL"),
		AdminPassword:    os.Getenv("SHELLY_ADMIN_PASSWORD"),
		CookieSecure:     os.Getenv("SHELLY_COOKIE_SECURE") == "true",
		OIDCIssuer:       os.Getenv("SHELLY_OIDC_ISSUER"),
		OIDCClientID:     os.Getenv("SHELLY_OIDC_CLIENT_ID"),
		OIDCClientSecret: os.Getenv("SHELLY_OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:  os.Getenv("SHELLY_OIDC_REDIRECT_URL"),
		HomeAssistant:    os.Getenv("SHELLY_HOME_ASSISTANT") == "true",
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

var optionName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// loadOptions applies a flat JSON object, such as the options.json that Home
// Assistant writes for apps, as SHELLY_<KEY> variables. Like dotenv files, it
// keeps variables that are already set. Empty and null options are skipped.
func loadOptions(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not read %s; check its permissions", path)
	}
	var options map[string]any
	// Errors never include the file's contents, which can contain passwords.
	if err := json.Unmarshal(data, &options); err != nil || options == nil {
		return fmt.Errorf("could not load %s; it must contain a JSON object", path)
	}
	for key, value := range options {
		if !optionName.MatchString(key) {
			return fmt.Errorf("option names in %s must be lowercase letters, digits, and underscores", path)
		}
		var text string
		switch value := value.(type) {
		case nil:
		case string:
			text = value
		case bool:
			text = strconv.FormatBool(value)
		case float64:
			text = strconv.FormatFloat(value, 'f', -1, 64)
		default:
			return fmt.Errorf("option %s in %s must be a string, number, or boolean", key, path)
		}
		name := "SHELLY_" + strings.ToUpper(key)
		if _, set := os.LookupEnv(name); set || text == "" {
			continue
		}
		if err := os.Setenv(name, text); err != nil {
			return err
		}
	}
	return nil
}
