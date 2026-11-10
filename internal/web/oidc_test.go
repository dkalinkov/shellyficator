package web

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"shellyficator/internal/domain"
)

type testOIDC struct {
	state, nonce, verifier string
	exchanges              int
	err                    error
}

func (p *testOIDC) AuthorizationURL(_ context.Context, state, nonce, verifier string) (string, error) {
	p.state, p.nonce, p.verifier = state, nonce, verifier
	return "https://auth.example.com/authorize?state=" + url.QueryEscape(state), p.err
}
func (p *testOIDC) Exchange(_ context.Context, code, nonce, verifier string) (domain.ExternalIdentity, error) {
	p.exchanges++
	if code != "authorization-code" || nonce != p.nonce || verifier != p.verifier {
		return domain.ExternalIdentity{}, errors.New("flow values changed")
	}
	return domain.ExternalIdentity{Issuer: "https://auth.example.com", Subject: "stable-subject", Email: "operator@example.com", EmailVerified: true}, p.err
}

func TestOIDCRoutesProtectAndConsumeBrowserBoundFlow(t *testing.T) {
	s, _ := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	if err := s.accounts.Setup("operator@example.com", "account-test-password"); err != nil {
		t.Fatal(err)
	}
	passwordSession, err := s.accounts.Authenticate("operator@example.com", "account-test-password")
	if err != nil {
		t.Fatal(err)
	}
	provider := &testOIDC{}
	s.oidc = provider
	s.oidcPending = map[string]oidcAttempt{}
	r, err := apis.NewRouter(s.store.App)
	if err != nil {
		t.Fatal(err)
	}
	static, err := fs.Sub(assets, "static")
	if err != nil {
		t.Fatal(err)
	}
	s.registerRoutes(r, static)
	mux, err := r.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	findCookie := func(response *httptest.ResponseRecorder, name string) *http.Cookie {
		t.Helper()
		for _, cookie := range response.Result().Cookies() {
			if cookie.Name == name {
				return cookie
			}
		}
		t.Fatalf("missing %s cookie", name)
		return nil
	}
	page := request("GET", "/login", nil)
	if !strings.Contains(page.Body.String(), "Sign in with Authelia") {
		t.Fatal("OIDC option missing")
	}
	csrf := findCookie(page, "shelly_csrf")
	if response := request("POST", "/login/authelia", nil); response.Code != 403 {
		t.Fatal("OIDC start bypassed CSRF")
	}
	start := func() *http.Cookie {
		t.Helper()
		response := request("POST", "/login/authelia", url.Values{"csrf": {csrf.Value}}, csrf)
		if response.Code != 303 || !strings.HasPrefix(response.Header().Get("Location"), "https://auth.example.com/") {
			t.Fatalf("start failed: %d", response.Code)
		}
		cookie := findCookie(response, "shelly_oidc")
		if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 600 {
			t.Fatal("incorrect flow cookie")
		}
		return cookie
	}
	callback := func(state string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		return request("GET", "/auth/oidc/callback?code=authorization-code&state="+url.QueryEscape(state), nil, cookies...)
	}
	cookie := start()
	for _, response := range []*httptest.ResponseRecorder{callback(provider.state), callback("incorrect", cookie)} {
		if response.Header().Get("Location") != "/login?oidc_error=failed" {
			t.Fatal("unbound callback accepted")
		}
	}
	if provider.exchanges != 0 {
		t.Fatal("unbound callback reached token endpoint")
	}
	response := callback(provider.state, cookie)
	if response.Header().Get("Location") != "/" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("callback failed")
	}
	session := findCookie(response, "shelly_session")
	operator, err := s.accounts.ValidateSession(session.Value)
	if err != nil || operator.ID != passwordSession.Operator.ID {
		t.Fatal("OIDC did not retain existing operator")
	}
	if callback(provider.state, cookie).Header().Get("Location") != "/login?oidc_error=failed" || provider.exchanges != 1 {
		t.Fatal("callback replay accepted")
	}
	cookie = start()
	pending := s.oidcPending[provider.state]
	pending.expires = time.Now().Add(-time.Second)
	s.oidcPending[provider.state] = pending
	if callback(provider.state, cookie).Header().Get("Location") != "/login?oidc_error=failed" || provider.exchanges != 1 {
		t.Fatal("expired flow accepted")
	}
	provider.err = errors.New("sensitive-provider-error")
	response = request("POST", "/login/authelia", url.Values{"csrf": {csrf.Value}}, csrf)
	if !strings.Contains(response.Body.String(), "Authelia is unavailable") || strings.Contains(response.Body.String(), provider.err.Error()) {
		t.Fatal("provider outage removed recovery or exposed details")
	}
	login := request("POST", "/login", url.Values{"csrf": {csrf.Value}, "email": {"operator@example.com"}, "password": {"account-test-password"}}, csrf)
	if login.Header().Get("Location") != "/" {
		t.Fatal("provider outage broke password login")
	}
}
