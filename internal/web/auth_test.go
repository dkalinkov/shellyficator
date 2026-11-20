package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"shellyficator/internal/accountstore"
)

type sessionAccounts struct{ Accounts }

func (sessionAccounts) Authenticate(string, string) (string, error) {
	return "test-session", nil
}

func TestPasswordLimitIsSharedByDashboardAndPocketBase(t *testing.T) {
	paths := []string{"/login", "/api/collections/operators/auth-with-password", "/api/collections/_superusers/auth-with-password"}
	for _, attackPath := range paths {
		t.Run(attackPath, func(t *testing.T) {
			app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
			if err := app.Bootstrap(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = app.ResetBootstrapState() })
			if err := app.RunAllMigrations(); err != nil {
				t.Fatal(err)
			}
			accountService := accountstore.New(app)
			const email, password = "operator@example.com", "test-password-long-enough"
			if err := accountService.Setup(email, password); err != nil {
				t.Fatal(err)
			}
			routes, err := apis.NewRouter(app)
			if err != nil {
				t.Fatal(err)
			}
			if err := Register(&core.ServeEvent{App: app, Router: routes}, nil, accountService, Options{}); err != nil {
				t.Fatal(err)
			}
			mux, err := routes.BuildMux()
			if err != nil {
				t.Fatal(err)
			}
			csrf := &http.Cookie{Name: "shelly_csrf", Value: strings.Repeat("a", 64)}
			request := func(path, clientIP, password string) *httptest.ResponseRecorder {
				t.Helper()
				form := url.Values{"email": {email}, "identity": {email}, "password": {password}, "csrf": {csrf.Value}}
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
				req.RemoteAddr = clientIP + ":1234"
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.AddCookie(csrf)
				response := httptest.NewRecorder()
				mux.ServeHTTP(response, req)
				return response
			}
			for range 10 {
				response := request(attackPath, "192.0.2.1", "incorrect-password")
				want := http.StatusBadRequest
				if attackPath == "/login" {
					want = http.StatusOK
				}
				if response.Code != want {
					t.Fatalf("initial failed attempt on %s: %d, want %d", attackPath, response.Code, want)
				}
			}
			for _, path := range paths {
				if response := request(path, "192.0.2.1", password); response.Code != http.StatusTooManyRequests {
					t.Fatalf("password limit bypassed through %s: %d", path, response.Code)
				}
				response := request(path, "192.0.2.2", password)
				want := http.StatusOK
				if path == "/login" {
					want = http.StatusSeeOther
				}
				if response.Code != want {
					t.Fatalf("independent client's legitimate login on %s failed: %d", path, response.Code)
				}
			}
		})
	}
}

func TestPasswordLimitExpiresAndSuccessfulLoginResetsAttempts(t *testing.T) {
	s := &Server{accounts: sessionAccounts{}, loginAttempts: map[string]loginAttempt{}}
	request := httptest.NewRequest(http.MethodPost, "/login", nil)
	ip := (&core.RequestEvent{Event: router.Event{Request: request}}).RemoteIP()
	login := func() {
		t.Helper()
		response := httptest.NewRecorder()
		event := &core.RequestEvent{Event: router.Event{Request: request.Clone(t.Context()), Response: response}}
		if err := s.login(event); err != nil || response.Code != http.StatusSeeOther {
			t.Fatalf("valid log in returned status %d, error %v", response.Code, err)
		}
	}
	s.loginAttempts[ip] = loginAttempt{count: 10, since: time.Now().Add(-16 * time.Minute)}
	login()
	s.loginAttempts[ip] = loginAttempt{count: 9, since: time.Now()}
	login()
	login()
}

func TestAuthCookiesUseConfiguredSecurityAndTLS(t *testing.T) {
	for _, test := range []struct {
		name, scheme string
		configured   bool
		secure       bool
	}{
		{"plain HTTP", "http", false, false},
		{"direct HTTPS", "https", false, true},
		{"HTTPS proxy", "http", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := &Server{accounts: sessionAccounts{}, cookieSecure: test.configured, loginAttempts: map[string]loginAttempt{}}
			for _, action := range []string{"csrf", "login", "logout"} {
				t.Run(action, func(t *testing.T) {
					response := httptest.NewRecorder()
					event := &core.RequestEvent{Event: router.Event{
						Request: httptest.NewRequest(http.MethodPost, test.scheme+"://example.com/"+action, nil), Response: response,
					}}
					var err error
					switch action {
					case "csrf":
						server.csrf(event)
					case "login":
						err = server.login(event)
					case "logout":
						err = server.logout(event)
					}
					if err != nil {
						t.Fatal(err)
					}
					cookies := response.Result().Cookies()
					if len(cookies) != 1 || cookies[0].Secure != test.secure {
						t.Fatalf("%s cookie security changed", action)
					}
				})
			}
		})
	}
}
