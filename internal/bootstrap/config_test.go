package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvFiles(t *testing.T) {
	const key = "SHELLYFICATOR_TEST_VALUE"
	for _, test := range []struct {
		name, defaults, local, environment, want string
		setEnvironment                           bool
	}{
		{name: "optional files are absent"},
		{name: "quoted values and comments", defaults: "# Configuration\nexport " + key + "='value with spaces # inside' # comment\n", want: "value with spaces # inside"},
		{name: "local overrides defaults", defaults: key + "=default\n", local: key + "=local\n", want: "local"},
		{name: "environment overrides files", defaults: key + "=default\n", local: key + "=local\n", setEnvironment: true, environment: "from-shell", want: "from-shell"},
		{name: "explicit empty environment is preserved", defaults: key + "=default\n", setEnvironment: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv(key, test.environment)
			if !test.setEnvironment {
				if err := os.Unsetenv(key); err != nil {
					t.Fatal(err)
				}
			}
			for path, contents := range map[string]string{".env": test.defaults, ".env.local": test.local} {
				if contents != "" {
					if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := loadEnv(); err != nil {
				t.Fatal(err)
			}
			if os.Getenv(key) != test.want {
				t.Fatal("configuration precedence or parsing was incorrect")
			}
		})
	}
}

func TestEnvErrorsDoNotExposeContents(t *testing.T) {
	for _, path := range []string{".env", ".env.local"} {
		t.Run(path, func(t *testing.T) {
			t.Chdir(t.TempDir())
			const secret = "example-secret-do-not-log"
			if err := os.WriteFile(path, []byte("INVALID!="+secret+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			err := loadEnv()
			if err == nil || !strings.Contains(err.Error(), path) || strings.Contains(err.Error(), secret) {
				t.Fatal("malformed configuration did not return a safe, actionable error")
			}
		})
	}
}

func TestUnreadableEnvIsAnError(t *testing.T) {
	for _, path := range []string{".env", ".env.local"} {
		t.Run(path, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.Mkdir(filepath.Clean(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := loadEnv(); err == nil {
				t.Fatal("unreadable configuration was treated as an optional missing file")
			}
		})
	}
}

func TestLoadConfigCapturesSettingsWithoutRequiringSetup(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, key := range []string{"SHELLY_ADMIN_EMAIL", "SHELLY_ADMIN_PASSWORD", "SHELLY_DEVICE_PASSWORD", "SHELLY_COOKIE_SECURE"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(".env", []byte("SHELLY_ADMIN_EMAIL=operator@example.com\nSHELLY_ADMIN_PASSWORD=default-password\nSHELLY_DEVICE_PASSWORD=device-default\nSHELLY_COOKIE_SECURE=true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".env.local", []byte("SHELLY_ADMIN_PASSWORD=local-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELLY_DEVICE_PASSWORD", "")
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.AdminEmail != "operator@example.com" || config.AdminPassword != "local-password" || config.DevicePassword != "" || !config.CookieSecure {
		t.Fatal("typed configuration did not preserve dotenv and process precedence")
	}
	t.Setenv("SHELLY_ADMIN_PASSWORD", "changed-after-loading")
	if config.AdminPassword != "local-password" {
		t.Fatal("configuration changed after it was loaded")
	}

	t.Chdir(t.TempDir())
	t.Setenv("SHELLY_ADMIN_EMAIL", "")
	t.Setenv("SHELLY_ADMIN_PASSWORD", "")
	if _, err := LoadConfig(); err != nil {
		t.Fatal("configuration required credentials for commands that do not use setup")
	}
}

func TestCookieSecureUsesTheExistingExplicitTrueValue(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, value := range []string{"", "false", "true", "TRUE", "1"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("SHELLY_COOKIE_SECURE", value)
			config, err := LoadConfig()
			if err != nil {
				t.Fatal(err)
			}
			if config.CookieSecure != (value == "true") {
				t.Fatal("secure-cookie environment behavior changed")
			}
		})
	}
}

func TestOptionsFileFillsUnsetSettings(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, key := range []string{"SHELLY_DEVICE_PASSWORD", "SHELLY_COOKIE_SECURE", "SHELLY_ADMIN_EMAIL", "SHELLY_ADMIN_PASSWORD", "SHELLY_OIDC_ISSUER"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	options := filepath.Join(t.TempDir(), "options.json")
	if err := os.WriteFile(options, []byte(`{"device_password":"from-options","cookie_secure":true,"admin_email":"","admin_password":null,"oidc_issuer":"https://options.example"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".env", []byte("SHELLY_DEVICE_PASSWORD=from-dotenv\nSHELLY_ADMIN_EMAIL=dotenv@example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELLY_OIDC_ISSUER", "https://environment.example")
	t.Setenv("SHELLY_OPTIONS_FILE", options)
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.DevicePassword != "from-options" || !config.CookieSecure {
		t.Fatal("options did not take precedence over dotenv files")
	}
	if config.OIDCIssuer != "https://environment.example" {
		t.Fatal("options replaced a value from the process environment")
	}
	if config.AdminEmail != "dotenv@example.com" || config.AdminPassword != "" {
		t.Fatal("empty or null options were not skipped")
	}
}

func TestOptionsFileFormatsScalars(t *testing.T) {
	const key = "SHELLY_TEST_OPTION"
	for _, test := range []struct{ json, want string }{
		{`"text"`, "text"}, {`false`, "false"}, {`8090`, "8090"}, {`0.5`, "0.5"}, {`1e21`, "1000000000000000000000"},
	} {
		t.Run(test.json, func(t *testing.T) {
			t.Setenv(key, "")
			if err := os.Unsetenv(key); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "options.json")
			if err := os.WriteFile(path, []byte(`{"test_option":`+test.json+`}`), 0600); err != nil {
				t.Fatal(err)
			}
			if err := loadOptions(path); err != nil {
				t.Fatal(err)
			}
			if got := os.Getenv(key); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestMissingOptionsFileIsAllowed(t *testing.T) {
	if err := loadOptions(filepath.Join(t.TempDir(), "options.json")); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidOptionsAreSafeErrors(t *testing.T) {
	const secret = "example-secret-do-not-log"
	for name, contents := range map[string]string{
		"syntax":       `{"device_password":"` + secret,
		"not object":   `["` + secret + `"]`,
		"null":         `null`,
		"nested value": `{"device_password":{"value":"` + secret + `"}}`,
		"list value":   `{"device_password":["` + secret + `"]}`,
		"unsafe name":  `{"device=password":"` + secret + `"}`,
		"uppercase":    `{"DEVICE_PASSWORD":"` + secret + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "options.json")
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			err := loadOptions(path)
			if err == nil || !strings.Contains(err.Error(), path) || strings.Contains(err.Error(), secret) {
				t.Fatalf("got %v; want an error naming the file without its contents", err)
			}
		})
	}
	t.Run("unreadable", func(t *testing.T) {
		if err := loadOptions(t.TempDir()); err == nil {
			t.Fatal("a directory was treated as a missing options file")
		}
	})
}
