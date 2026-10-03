package web

import (
	"context"
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
	Capture(context.Context, string) (domain.Device, error)
	ApproveBaseline(context.Context, string) (domain.Device, error)
	OrganizationOptions() (domain.OrganizationOptions, error)
	Metadata(string, string, string, string) error
	Organization(string, string, string) error
	SaveChannelNote(string, string, string) error
	LocalNetworks() ([]domain.Network, error)
	Discover(string, bool) error
	Discovery() domain.Discovery
	Refresh(bool) <-chan struct{}
}

type Accounts interface {
	NeedsSetup() (bool, error)
	Setup(string, string) error
	Authenticate(string, string) (domain.Session, error)
	ValidateSession(string) (domain.Operator, error)
}

type Server struct {
	devices       Devices
	accounts      Accounts
	templates     *template.Template
	version       string
	authMu        sync.Mutex
	loginAttempts map[string]loginAttempt
}

func Register(e *core.ServeEvent, devices Devices, accounts Accounts, version string) error {
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
	s := &Server{devices: devices, accounts: accounts, templates: templates, version: version, loginAttempts: map[string]loginAttempt{}}
	s.registerRoutes(e.Router, static)
	return nil
}

func (s *Server) redirect(e *core.RequestEvent, path, notice string) error {
	if notice != "" {
		path += "?notice=" + url.QueryEscape(notice)
	}
	return e.Redirect(http.StatusSeeOther, path)
}
