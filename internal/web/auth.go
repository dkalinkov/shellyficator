package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
)

type loginAttempt struct {
	count int
	since time.Time
}

func (s *Server) loginPage(e *core.RequestEvent) error {
	needsSetup, err := s.accounts.NeedsSetup()
	if err != nil {
		return err
	}
	if needsSetup {
		return e.Redirect(http.StatusSeeOther, "/setup")
	}
	message := ""
	if e.Request.URL.Query().Get("oidc_error") != "" {
		message = "Authelia sign-in failed. Try again or use your password."
	}
	return s.renderLogin(e, message)
}

func (s *Server) renderLogin(e *core.RequestEvent, message string) error {
	e.Response.Header().Set("Cache-Control", "no-store")
	return s.render(e, "login", AuthPage{PageMeta: s.pageMeta(e, "Sign in"), Error: message, OIDCEnabled: s.oidc != nil})
}

func (s *Server) csrf(e *core.RequestEvent) string {
	if cookie, err := e.Request.Cookie("shelly_csrf"); err == nil && len(cookie.Value) == 64 {
		return cookie.Value
	}
	var token [32]byte
	_, _ = rand.Read(token[:])
	value := hex.EncodeToString(token[:])
	http.SetCookie(e.Response, &http.Cookie{Name: "shelly_csrf", Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.secureCookie(e), MaxAge: 86400})
	return value
}

func (s *Server) secureCookie(e *core.RequestEvent) bool {
	return e.Request.TLS != nil || s.cookieSecure
}

func checkCSRF(e *core.RequestEvent) error {
	e.Request.Body = http.MaxBytesReader(e.Response, e.Request.Body, 32768)
	if err := e.Request.ParseForm(); err != nil {
		return e.BadRequestError("Invalid form", nil)
	}
	cookie, err := e.Request.Cookie("shelly_csrf")
	token := e.Request.FormValue("csrf")
	if err != nil || len(token) != 64 || subtle.ConstantTimeCompare([]byte(token), []byte(cookie.Value)) != 1 {
		return e.ForbiddenError("Refresh the page and try again", nil)
	}
	if origin := e.Request.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != e.Request.Host {
			return e.ForbiddenError("Invalid request origin", nil)
		}
	}
	return e.Next()
}

func (s *Server) requireLogin(e *core.RequestEvent) error {
	e.Response.Header().Set("Cache-Control", "no-store")
	e.Response.Header().Set("Referrer-Policy", "same-origin")
	e.Response.Header().Set("X-Content-Type-Options", "nosniff")
	cookie, err := e.Request.Cookie("shelly_session")
	if err == nil {
		_, err := s.accounts.ValidateSession(cookie.Value)
		if err == nil {
			return e.Next()
		}
	}
	if e.Request.Header.Get("HX-Request") == "true" {
		e.Response.Header().Set("HX-Redirect", "/login")
		return e.NoContent(401)
	}
	return e.Redirect(http.StatusSeeOther, "/login")
}

func (s *Server) login(e *core.RequestEvent) error {
	ip := e.RemoteIP()
	s.authMu.Lock()
	for key, attempt := range s.loginAttempts {
		if time.Since(attempt.since) > 15*time.Minute {
			delete(s.loginAttempts, key)
		}
	}
	attempt := s.loginAttempts[ip]
	if attempt.since.IsZero() {
		attempt.since = time.Now()
	}
	attempt.count++
	s.loginAttempts[ip] = attempt
	s.authMu.Unlock()
	if attempt.count > 10 {
		return e.TooManyRequestsError("Too many sign in attempts. Try again in 15 minutes.", nil)
	}
	session, err := s.accounts.Authenticate(strings.TrimSpace(e.Request.FormValue("email")), e.Request.FormValue("password"))
	if err != nil {
		if !errors.Is(err, domain.ErrInvalidCredentials) {
			return e.InternalServerError("Could not sign in", nil)
		}
		e.Response.Header().Set("Cache-Control", "no-store")
		return s.renderLogin(e, "Email or password is incorrect.")
	}
	s.authMu.Lock()
	delete(s.loginAttempts, ip)
	s.authMu.Unlock()
	s.setSession(e, session.Token)
	return e.Redirect(http.StatusSeeOther, "/")
}

func (s *Server) setSession(e *core.RequestEvent, token string) {
	http.SetCookie(e.Response, &http.Cookie{Name: "shelly_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.secureCookie(e), MaxAge: 86400})
}

func (s *Server) setupPage(e *core.RequestEvent) error {
	needsSetup, err := s.accounts.NeedsSetup()
	if err != nil {
		return err
	}
	if !needsSetup {
		return e.Redirect(http.StatusSeeOther, "/login")
	}
	return s.render(e, "setup", AuthPage{PageMeta: s.pageMeta(e, "Create your account")})
}

func (s *Server) setup(e *core.RequestEvent) error {
	email, password := strings.TrimSpace(e.Request.FormValue("email")), e.Request.FormValue("password")
	if len(password) < 12 {
		return s.render(e, "setup", AuthPage{PageMeta: s.pageMeta(e, "Create your account"), Error: "Choose a password with at least 12 characters."})
	}
	err := s.accounts.Setup(email, password)
	if err != nil {
		return s.render(e, "setup", AuthPage{PageMeta: s.pageMeta(e, "Create your account"), Error: "Could not create the account. Check the email and password, or sign in if setup is already complete."})
	}
	return s.redirect(e, "/login", "Account created.")
}

func (s *Server) logout(e *core.RequestEvent) error {
	http.SetCookie(e.Response, &http.Cookie{Name: "shelly_session", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.secureCookie(e), MaxAge: -1})
	return e.Redirect(http.StatusSeeOther, "/login")
}
