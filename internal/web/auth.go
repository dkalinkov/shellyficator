package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type loginAttempt struct {
	count int
	since time.Time
}

func (s *Server) loginPage(e *core.RequestEvent) error {
	count, err := s.store.App.CountRecords("operators")
	if err != nil {
		return err
	}
	if count == 0 {
		return e.Redirect(http.StatusSeeOther, "/setup")
	}
	return s.render(e, "login", Page{Title: "Sign in", CSRF: csrf(e)})
}

func csrf(e *core.RequestEvent) string {
	if cookie, err := e.Request.Cookie("shelly_csrf"); err == nil && len(cookie.Value) == 64 {
		return cookie.Value
	}
	var token [32]byte
	_, _ = rand.Read(token[:])
	value := hex.EncodeToString(token[:])
	http.SetCookie(e.Response, &http.Cookie{Name: "shelly_csrf", Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: secureCookie(e), MaxAge: 86400})
	return value
}

func secureCookie(e *core.RequestEvent) bool {
	return e.Request.TLS != nil || os.Getenv("SHELLY_COOKIE_SECURE") == "true"
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
		record, err := s.store.App.FindAuthRecordByToken(cookie.Value, core.TokenTypeAuth)
		if err == nil && record.Collection().Name == "operators" {
			e.Auth = record
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
	s.mu.Lock()
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
	s.mu.Unlock()
	if attempt.count > 10 {
		return e.TooManyRequestsError("Too many sign in attempts. Try again in 15 minutes.", nil)
	}
	record, err := s.store.App.FindAuthRecordByEmail("operators", strings.TrimSpace(e.Request.FormValue("email")))
	if err != nil || !record.ValidatePassword(e.Request.FormValue("password")) {
		e.Response.Header().Set("Cache-Control", "no-store")
		return s.render(e, "login", Page{Title: "Sign in", CSRF: csrf(e), Error: "Email or password is incorrect."})
	}
	token, err := record.NewAuthToken()
	if err != nil {
		return e.InternalServerError("Could not sign in", nil)
	}
	s.mu.Lock()
	delete(s.loginAttempts, ip)
	s.mu.Unlock()
	http.SetCookie(e.Response, &http.Cookie{Name: "shelly_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secureCookie(e), MaxAge: 86400})
	return e.Redirect(http.StatusSeeOther, "/")
}

func (s *Server) setupPage(e *core.RequestEvent) error {
	count, err := s.store.App.CountRecords("operators")
	if err != nil {
		return err
	}
	if count > 0 {
		return e.Redirect(http.StatusSeeOther, "/login")
	}
	return s.render(e, "setup", Page{Title: "Create your account", CSRF: csrf(e)})
}

func (s *Server) setup(e *core.RequestEvent) error {
	email, password := strings.TrimSpace(e.Request.FormValue("email")), e.Request.FormValue("password")
	if len(password) < 12 {
		return s.render(e, "setup", Page{Title: "Create your account", CSRF: csrf(e), Error: "Choose a password with at least 12 characters."})
	}
	err := s.store.App.RunInTransaction(func(app core.App) error {
		count, err := app.CountRecords("operators")
		if err != nil {
			return err
		}
		if count > 0 {
			return errors.New("setup already completed")
		}
		collection, err := app.FindCollectionByNameOrId("operators")
		if err != nil {
			return err
		}
		record := core.NewRecord(collection)
		record.SetEmail(email)
		record.SetPassword(password)
		record.SetVerified(true)
		if err := app.Save(record); err != nil {
			return err
		}
		count, err = app.CountRecords(core.CollectionNameSuperusers, dbx.Not(dbx.HashExp{"email": core.DefaultInstallerEmail}))
		if err != nil {
			return err
		}
		if count == 0 {
			collection, err = app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
			if err != nil {
				return err
			}
			admin := core.NewRecord(collection)
			admin.SetEmail(email)
			admin.SetPassword(password)
			admin.SetVerified(true)
			if err := app.Save(admin); err != nil {
				return err
			}
			if installer, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, core.DefaultInstallerEmail); err == nil {
				return app.Delete(installer)
			}
			return nil
		}
		return nil
	})
	if err != nil {
		return s.render(e, "setup", Page{Title: "Create your account", CSRF: csrf(e), Error: "Could not create the account. Check the email and password, or sign in if setup is already complete."})
	}
	return s.redirect(e, "/login", "Account created.")
}

func (s *Server) logout(e *core.RequestEvent) error {
	http.SetCookie(e.Response, &http.Cookie{Name: "shelly_session", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secureCookie(e), MaxAge: -1})
	return e.Redirect(http.StatusSeeOther, "/login")
}
