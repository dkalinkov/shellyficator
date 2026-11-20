package web

import (
	"context"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"sync"

	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/domain"
)

// Devices describes application operations consumed by the HTML adapter.
type Devices interface {
	Inventory() ([]domain.InventoryEntry, error)
	Detail(string) (domain.DeviceDetails, error)
	RefreshDevice(context.Context, string) (domain.RefreshResult, error)
	CheckFirmware(context.Context, string) (domain.Device, error)
	StartFirmwareUpdates(context.Context, []domain.FirmwareSelection) error
	FirmwareRunning() bool
	Capture(context.Context, string) (domain.Device, error)
	ApproveBaseline(context.Context, string) (domain.Device, error)
	OrganizationOptions() (domain.OrganizationOptions, error)
	Metadata(string, string, string, string) error
	Organization(string, string, string) error
	SaveChannelNote(string, string, string) error
	LocalNetworks() ([]domain.Network, error)
	DiscoveryNetworks() ([]string, error)
	ForgetNetwork(string) error
	Discover(string, bool) error
	Discovery() domain.Discovery
	Refresh(bool) <-chan struct{}
}

type Accounts interface {
	NeedsSetup() (bool, error)
	Setup(string, string) error
	Authenticate(string, string) (string, error)
	AuthenticateIdentity(domain.ExternalIdentity) (string, error)
	ValidateSession(string) (operatorID string, err error)
}

type OIDC interface {
	AuthorizationURL(context.Context, string, string, string) (string, error)
	Exchange(context.Context, string, string, string) (domain.ExternalIdentity, error)
}

// Options contains presentation configuration supplied by the application.
type WatchStore interface {
	WatchPreferences() (domain.WatchPreferences, error)
	SaveWatchPolicy(string, domain.WatchPolicy) error
}

type Options struct {
	WatchStore    WatchStore
	Version       string
	CookieSecure  bool
	OIDC          OIDC
	HomeAssistant bool
}

type Server struct {
	watchStore    WatchStore
	devices       Devices
	accounts      Accounts
	templates     *template.Template
	templatesMu   sync.Mutex
	baseTemplates map[string]*template.Template
	version       string
	cookieSecure  bool
	homeAssistant bool
	authMu        sync.Mutex
	loginAttempts map[string]loginAttempt
	oidc          OIDC
	oidcPending   map[string]oidcAttempt
}

func Register(e *core.ServeEvent, devices Devices, accounts Accounts, options Options) error {
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
	s := &Server{watchStore: options.WatchStore, devices: devices, accounts: accounts, templates: templates, version: options.Version, cookieSecure: options.CookieSecure, homeAssistant: options.HomeAssistant, loginAttempts: map[string]loginAttempt{}, oidc: options.OIDC, oidcPending: map[string]oidcAttempt{}}
	s.registerPasswordLimiter(e.App)
	s.registerRoutes(e.Router, static)
	return nil
}

func (s *Server) redirect(e *core.RequestEvent, path, notice string, level ...string) error {
	if notice != "" {
		path += "?notice=" + url.QueryEscape(notice)
		if len(level) > 0 {
			path += "&notice_level=" + url.QueryEscape(level[0])
		}
	}
	return e.Redirect(http.StatusSeeOther, s.base(e)+path)
}

func (s *Server) notify(e *core.RequestEvent, message, level string) {
	if message == "" {
		return
	}
	trigger, _ := json.Marshal(map[string]any{"notify": map[string]string{
		"message": message, "level": level, "target": "#toast-stack",
	}})
	e.Response.Header().Set("HX-Trigger", string(trigger))
}
