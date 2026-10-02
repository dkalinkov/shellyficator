package web

import (
	"bytes"
	"context"
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/inventory"
	"shellyficator/internal/shelly"
)

//go:embed templates/*.html static/*
var assets embed.FS

type Server struct {
	store               *inventory.Store
	client              *shelly.Client
	announcements       func(context.Context) ([]string, error)
	neighbors           func(context.Context) ([]string, error)
	templates           *template.Template
	version             string
	mu                  sync.Mutex
	inventoryMu         sync.RWMutex
	discovery           Discovery
	refreshing          bool
	lastRefresh         time.Time
	refreshDone         chan struct{}
	refreshForced       bool
	refreshForcePending bool
	loginAttempts       map[string]loginAttempt
}

type Device struct {
	Record *core.Record
	Name   string
	Table  DeviceTable
}
type Metric struct{ Label, Value string }
type RoomGroup struct {
	Room    string
	Devices []Device
}
type Page struct {
	Title, CSRF, Notice, Error, Query, Room string
	GroupBy                                 string
	Version                                 string
	Devices                                 []Device
	RoomGroups                              []RoomGroup
	Rooms, Tags                             []string
	SelectedTags                            []string
	Total, Reachable, Changed, Unprotected  int
	Discovery                               Discovery
	Networks                                []shelly.Network
	Device                                  *core.Record
	Name                                    string
	Detail                                  DeviceDetail
	Differences                             []inventory.Difference
	HasBaseline                             bool
	Snapshots                               []*core.Record
}

func Register(e *core.ServeEvent, store *inventory.Store, client *shelly.Client, version string) error {
	// The app's first-run form creates both accounts; the PocketBase installer is redundant.
	e.InstallerFunc = nil
	templates, err := parseTemplates()
	if err != nil {
		return err
	}
	s := &Server{store: store, client: client, announcements: shelly.Announcements, neighbors: shelly.Neighbors, templates: templates, version: version, loginAttempts: map[string]loginAttempt{}}
	static, err := fs.Sub(assets, "static")
	if err != nil {
		return err
	}
	e.Router.GET("/assets/{path...}", apis.Static(static, false))
	e.Router.GET("/healthz", func(e *core.RequestEvent) error { return e.String(200, "ok") })
	e.Router.GET("/favicon.ico", func(e *core.RequestEvent) error { return e.NoContent(204) })
	e.Router.GET("/login", s.loginPage)
	e.Router.GET("/setup", s.setupPage)
	e.Router.POST("/setup", s.setup).BindFunc(checkCSRF)
	e.Router.POST("/login", s.login).BindFunc(checkCSRF)
	protected := e.Router.Group("")
	protected.BindFunc(s.requireLogin)
	protected.GET("/{$}", s.dashboard)
	protected.GET("/inventory", s.table)
	protected.GET("/discovery/progress", s.progress)
	protected.POST("/logout", s.logout).BindFunc(checkCSRF)
	protected.POST("/discover", s.discover).BindFunc(checkCSRF)
	protected.GET("/devices/{id}", s.detail)
	protected.POST("/devices/{id}/refresh", s.refreshDevice).BindFunc(checkCSRF)
	protected.GET("/devices/{id}/organization", s.organizationEditor)
	protected.POST("/devices/{id}/organization", s.organization).BindFunc(checkCSRF)
	protected.POST("/devices/{id}/metadata", s.metadata).BindFunc(checkCSRF)
	protected.POST("/devices/{id}/channel-notes", s.channelNote).BindFunc(checkCSRF)
	protected.POST("/devices/{id}/capture", s.capture).BindFunc(checkCSRF)
	protected.POST("/devices/{id}/baseline", s.baseline).BindFunc(checkCSRF)
	return nil
}

func parseTemplates() (*template.Template, error) {
	functions := template.FuncMap{
		"get":   func(r *core.Record, key string) string { return r.GetString(key) },
		"flag":  func(r *core.Record, key string) bool { return r.GetBool(key) },
		"when":  inventory.FormatTime,
		"value": inventory.Display,
		"tags":  inventory.SplitTags,
	}
	return template.New("app").Funcs(functions).ParseFS(assets, "templates/*.html")
}

func (s *Server) render(e *core.RequestEvent, name string, page Page) error {
	page.Version = s.version
	var buffer bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buffer, name, page); err != nil {
		return err
	}
	return e.HTML(200, buffer.String())
}

func (s *Server) redirect(e *core.RequestEvent, path, notice string) error {
	if notice != "" {
		path += "?notice=" + url.QueryEscape(notice)
	}
	return e.Redirect(http.StatusSeeOther, path)
}
