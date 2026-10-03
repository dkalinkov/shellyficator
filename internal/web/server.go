package web

import (
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"sync"

	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/devices"
	"shellyficator/internal/inventory"
	"shellyficator/internal/shelly"
)

type Server struct {
	store         *inventory.Store
	client        *shelly.Client
	devices       *devices.Service
	templates     *template.Template
	version       string
	authMu        sync.Mutex
	loginAttempts map[string]loginAttempt
}

func Register(e *core.ServeEvent, store *inventory.Store, client *shelly.Client, version string) error {
	// The app's first-run form creates both accounts; the PocketBase installer is redundant.
	e.InstallerFunc = nil
	templates, err := parseTemplates()
	if err != nil {
		return err
	}
	static, err := fs.Sub(assets, "static")
	if err != nil {
		return err
	}
	s := &Server{store: store, client: client, devices: devices.New(store, client), templates: templates, version: version, loginAttempts: map[string]loginAttempt{}}
	e.App.OnTerminate().BindFunc(func(event *core.TerminateEvent) error {
		s.devices.Close()
		return event.Next()
	})
	s.registerRoutes(e.Router, static)
	return nil
}

func (s *Server) redirect(e *core.RequestEvent, path, notice string) error {
	if notice != "" {
		path += "?notice=" + url.QueryEscape(notice)
	}
	return e.Redirect(http.StatusSeeOther, path)
}
