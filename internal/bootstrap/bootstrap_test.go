package bootstrap

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/accounts"
	"shellyficator/internal/accountstore"
	"shellyficator/internal/domain"
	"shellyficator/internal/inventory"
)

func testApp(t *testing.T, args []string, config Config, build BuildInfo) *pocketbase.PocketBase {
	t.Helper()
	originalArgs := os.Args
	os.Args = append([]string{"shellyficator", "--dir", t.TempDir()}, args...)
	t.Cleanup(func() { os.Args = originalArgs })
	app := New(config, build)
	app.RootCmd.SetArgs(os.Args[1:])
	app.RootCmd.SetOut(io.Discard)
	t.Cleanup(func() {
		if !app.IsBootstrapped() {
			return
		}
		if err := app.OnTerminate().Trigger(&core.TerminateEvent{App: app}, func(event *core.TerminateEvent) error {
			return event.App.ClearBootstrap()
		}); err != nil {
			t.Error(err)
		}
	})
	return app
}

func TestVersionDoesNotInitializeStorageOrServing(t *testing.T) {
	for _, build := range []BuildInfo{
		{Version: "0.1.0-dev", Commit: "unknown", BuildDate: "unknown"},
		{Version: "0.1.0", Commit: "abc123", BuildDate: "2026-10-03T10:00:00Z"},
	} {
		t.Run(build.Version, func(t *testing.T) {
			app := testApp(t, []string{"--version"}, Config{AdminPassword: "short"}, build)
			app.OnServe().BindFunc(func(event *core.ServeEvent) error {
				t.Error("version initialized serving dependencies")
				return event.Next()
			})
			var output bytes.Buffer
			app.RootCmd.SetOut(&output)
			if err := app.Start(); err != nil {
				t.Fatal(err)
			}
			want := "Shellyficator v0.1.0-dev\n"
			if build.Version == "0.1.0" {
				want = "Shellyficator v0.1.0 (commit abc123, built 2026-10-03T10:00:00Z)\n"
			}
			if output.String() != want {
				t.Fatalf("version output changed: %q", output.String())
			}
			files, err := os.ReadDir(app.DataDir())
			if err != nil || len(files) != 0 || app.IsBootstrapped() {
				t.Fatalf("version opened storage: %d files, %v", len(files), err)
			}
		})
	}
}

func TestSetupUsesConfigurationAndDoesNotStartServing(t *testing.T) {
	config := Config{AdminEmail: "operator@example.com", AdminPassword: "bootstrap-test-password"}
	app := testApp(t, []string{"setup"}, config, BuildInfo{Version: "test"})
	t.Setenv("SHELLY_ADMIN_EMAIL", "changed@example.com")
	t.Setenv("SHELLY_ADMIN_PASSWORD", "short")
	app.OnServe().BindFunc(func(event *core.ServeEvent) error {
		t.Error("setup initialized serving dependencies")
		return event.Next()
	})
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	if app.IsBootstrapped() {
		t.Fatal("setup did not release storage on command completion")
	}
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	for _, collection := range []string{"operators", core.CollectionNameSuperusers} {
		if _, err := app.FindAuthRecordByEmail(collection, config.AdminEmail); err != nil {
			t.Fatalf("setup did not persist the configured account in %s: %v", collection, err)
		}
	}
}

func TestSetupValidatesCredentialsOnlyWhenExecuted(t *testing.T) {
	app := testApp(t, []string{"setup"}, Config{AdminPassword: "short"}, BuildInfo{Version: "test"})
	err := app.Start()
	if err == nil || !strings.Contains(err.Error(), "set SHELLY_ADMIN_EMAIL and SHELLY_ADMIN_PASSWORD (at least 12 characters)") {
		t.Fatalf("setup validation changed: %v", err)
	}
	if app.IsBootstrapped() {
		t.Fatal("failed setup did not release storage")
	}
}

func TestServingUsesPresentationConfiguration(t *testing.T) {
	t.Setenv("SHELLY_COOKIE_SECURE", "false")
	app := testApp(t, nil, Config{CookieSecure: true}, BuildInfo{Version: "bootstrap-version"})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	router, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.OnServe().Trigger(&core.ServeEvent{App: app, Router: router}); err != nil {
		t.Fatal(err)
	}
	mux, err := router.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/setup", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "bootstrap-version") {
		t.Fatalf("setup did not render the configured version: %d", response.Code)
	}
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "shelly_csrf" {
			if !cookie.Secure {
				t.Fatal("configured secure cookies were lost between bootstrap and presentation")
			}
			return
		}
	}
	t.Fatal("setup did not issue a CSRF cookie")
}

func TestServingLifecycleDrainsWorkBeforeStorageCloses(t *testing.T) {
	for _, failStartup := range []bool{false, true} {
		name := "termination"
		if failStartup {
			name = "startup failure"
		}
		t.Run(name, func(t *testing.T) {
			started, cancelled := make(chan struct{}), make(chan struct{})
			device := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/shelly" {
					t.Errorf("unexpected device request: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				close(started)
				<-r.Context().Done()
				close(cancelled)
			}))
			t.Cleanup(device.Close)
			app := testApp(t, nil, Config{}, BuildInfo{Version: "bootstrap-test"})
			if err := app.Bootstrap(); err != nil {
				t.Fatal(err)
			}
			if err := app.RunAllMigrations(); err != nil {
				t.Fatal(err)
			}
			if err := accounts.New(accountstore.New(app)).Setup("operator@example.com", "bootstrap-test-password"); err != nil {
				t.Fatal(err)
			}
			operator, err := app.FindAuthRecordByEmail("operators", "operator@example.com")
			if err != nil {
				t.Fatal(err)
			}
			token, err := operator.NewAuthToken()
			if err != nil {
				t.Fatal(err)
			}
			store := inventory.New(app)
			record, err := store.SaveReading(domain.Reading{
				Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4},
				Address:  device.URL, Status: map[string]any{"switch:0": map[string]any{"output": false}},
			}, false)
			if err != nil {
				t.Fatal(err)
			}
			router, err := apis.NewRouter(app)
			if err != nil {
				t.Fatal(err)
			}
			requestDone := make(chan struct{})
			startRequest := func() {
				mux, err := router.BuildMux()
				if err != nil {
					t.Fatal(err)
				}
				for path, status := range map[string]int{"/healthz": 200, "/api/health": 200, "/": 303} {
					response := httptest.NewRecorder()
					mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
					if response.Code != status {
						t.Fatalf("composed route %s returned %d, want %d", path, response.Code, status)
					}
				}
				request := httptest.NewRequest(http.MethodGet, "/inventory/refresh?force=1", nil)
				request.AddCookie(&http.Cookie{Name: "shelly_session", Value: token})
				go func() {
					defer close(requestDone)
					mux.ServeHTTP(httptest.NewRecorder(), request)
				}()
				wait(t, started, "device request did not start")
			}
			wantFailure := errors.New("later startup hook failed")
			event := &core.ServeEvent{App: app, Router: router}
			if failStartup {
				err = app.OnServe().Trigger(event, func(event *core.ServeEvent) error {
					startRequest()
					return wantFailure
				})
				if !errors.Is(err, wantFailure) {
					t.Fatalf("startup error was lost: %v", err)
				}
				wait(t, cancelled, "failed startup did not cancel the device request")
				wait(t, requestDone, "failed startup did not drain the refresh handler")
				if err := app.OnServe().Trigger(&core.ServeEvent{App: app}); !errors.Is(err, domain.ErrClosed) {
					t.Fatalf("application served after failed startup: %v", err)
				}
			} else {
				if err := app.OnServe().Trigger(event); err != nil {
					t.Fatal(err)
				}
				startRequest()
			}
			app.OnTerminate().BindFunc(func(event *core.TerminateEvent) error {
				device, err := store.Device(record.ID)
				if err != nil || device.ConsecutiveFailures != 0 || !device.Reachable {
					t.Errorf("cancelled work changed device health before storage closed: %+v, %v", device, err)
				}
				return event.Next()
			})
			if err := app.OnTerminate().Trigger(&core.TerminateEvent{App: app}, func(event *core.TerminateEvent) error {
				return event.App.ClearBootstrap()
			}); err != nil {
				t.Fatal(err)
			}
			wait(t, cancelled, "shutdown did not cancel the device request")
			wait(t, requestDone, "shutdown did not drain the refresh handler")
			if app.IsBootstrapped() {
				t.Fatal("termination did not close storage")
			}
			// Termination and failed startup must not admit new serving work.
			if err := app.OnServe().Trigger(&core.ServeEvent{App: app}); !errors.Is(err, domain.ErrClosed) {
				t.Fatalf("application served after termination: %v", err)
			}
		})
	}
}

func wait(t *testing.T, done <-chan struct{}, problem string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal(problem)
	}
}
