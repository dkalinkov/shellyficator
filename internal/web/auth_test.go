package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"shellyficator/internal/domain"
)

type sessionAccounts struct{ Accounts }

func (sessionAccounts) Authenticate(string, string) (domain.Session, error) {
	return domain.Session{Token: "test-session"}, nil
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
		{"configured HTTPS", "https", true, true},
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
