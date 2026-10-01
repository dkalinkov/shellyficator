package main

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
