package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

const oidcLifetime = 10 * time.Minute

type oidcAttempt struct {
	nonce, verifier string
	expires         time.Time
}

func oidcRandom() string {
	var value [32]byte
	_, _ = rand.Read(value[:])
	return base64.RawURLEncoding.EncodeToString(value[:])
}

func (s *Server) oidcLogin(e *core.RequestEvent) error {
	e.Response.Header().Set("Cache-Control", "no-store")
	e.Response.Header().Set("Referrer-Policy", "no-referrer")
	state := oidcRandom()
	attempt := oidcAttempt{nonce: oidcRandom(), verifier: oidcRandom(), expires: time.Now().Add(oidcLifetime)}
	ctx, cancel := context.WithTimeout(e.Request.Context(), 15*time.Second)
	defer cancel()
	target, err := s.oidc.AuthorizationURL(ctx, state, attempt.nonce, attempt.verifier)
	if err != nil {
		return s.renderLogin(e, "Authelia is unavailable. Try again or use your password.")
	}
	s.authMu.Lock()
	for key, pending := range s.oidcPending {
		if !time.Now().Before(pending.expires) {
			delete(s.oidcPending, key)
		}
	}
	if cookie, err := e.Request.Cookie("shelly_oidc"); err == nil {
		delete(s.oidcPending, cookie.Value)
	}
	if len(s.oidcPending) >= 512 {
		s.authMu.Unlock()
		return e.TooManyRequestsError("Too many pending sign-ins. Try again shortly or use your password.", nil)
	}
	s.oidcPending[state] = attempt
	s.authMu.Unlock()
	http.SetCookie(e.Response, &http.Cookie{Name: "shelly_oidc", Value: state, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.secureCookie(e), MaxAge: int(oidcLifetime.Seconds())})
	return e.Redirect(http.StatusSeeOther, target)
}

func (s *Server) oidcCallback(e *core.RequestEvent) error {
	e.Response.Header().Set("Cache-Control", "no-store")
	e.Response.Header().Set("Referrer-Policy", "no-referrer")
	fail := func() error { return s.redirect(e, "/login?oidc_error=failed", "") }
	query := e.Request.URL.Query()
	state := query.Get("state")
	cookie, err := e.Request.Cookie("shelly_oidc")
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) != 1 {
		return fail()
	}
	s.authMu.Lock()
	attempt, ok := s.oidcPending[state]
	delete(s.oidcPending, state)
	s.authMu.Unlock()
	http.SetCookie(e.Response, &http.Cookie{Name: "shelly_oidc", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.secureCookie(e), MaxAge: -1})
	if !ok || !time.Now().Before(attempt.expires) || query.Get("error") != "" || query.Get("code") == "" {
		return fail()
	}
	ctx, cancel := context.WithTimeout(e.Request.Context(), 15*time.Second)
	defer cancel()
	identity, err := s.oidc.Exchange(ctx, query.Get("code"), attempt.nonce, attempt.verifier)
	if err != nil {
		return fail()
	}
	session, err := s.accounts.AuthenticateIdentity(identity)
	if err != nil {
		return fail()
	}
	s.setSession(e, session.Token)
	return s.redirect(e, "/", "")
}
