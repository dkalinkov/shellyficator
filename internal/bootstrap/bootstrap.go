// Package bootstrap assembles concrete application dependencies and owns their
// lifecycle. Other application layers must not depend on this package.
package bootstrap

import (
	"sync"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/accounts"
	"shellyficator/internal/accountstore"
	"shellyficator/internal/devices"
	"shellyficator/internal/domain"
	"shellyficator/internal/inventory"
	"shellyficator/internal/oidc"
	"shellyficator/internal/shelly"
	"shellyficator/internal/web"
	_ "shellyficator/migrations"
)

// New configures an application without opening storage or starting device work.
// PocketBase.Start owns command execution, database initialization, and signals.
func New(config Config, build BuildInfo) *pocketbase.PocketBase {
	app := pocketbase.New()
	registerCommands(app, config, build)

	// Serving dependencies are created lazily and shared by all serving adapters.
	// Protect publication against termination, including interrupted startup.
	var mu sync.Mutex
	var deviceService *devices.Service
	var terminated bool
	closeServices := func() {
		mu.Lock()
		terminated = true
		service := deviceService
		mu.Unlock()
		if service != nil {
			service.Close()
		}
	}
	app.OnTerminate().BindFunc(func(event *core.TerminateEvent) error {
		closeServices()
		return event.Next()
	})
	app.OnServe().BindFunc(func(event *core.ServeEvent) error {
		var login *oidc.Client
		if !config.HomeAssistant {
			var err error
			login, err = oidc.New(oidc.Config{Issuer: config.OIDCIssuer, ClientID: config.OIDCClientID, ClientSecret: config.OIDCClientSecret, RedirectURL: config.OIDCRedirectURL})
			if err != nil {
				return err
			}
		}
		mu.Lock()
		if terminated {
			mu.Unlock()
			return domain.ErrClosed
		}
		if deviceService == nil {
			store := inventory.New(app)
			client := shelly.NewClient(config.DevicePassword)
			deviceService = devices.New(store, client, devices.Options{
				Discovery: shelly.NetworkDiscovery{}, Logger: app.Logger(),
			})
		}
		service := deviceService
		mu.Unlock()
		accountService := accounts.New(accountstore.New(app))
		options := web.Options{WatchStore: inventory.New(app), Version: build.Version, CookieSecure: config.CookieSecure, HomeAssistant: config.HomeAssistant}
		if login != nil {
			options.OIDC = login
		}
		if err := web.Register(event, service, accountService, options); err != nil {
			closeServices()
			return err
		}
		if err := event.Next(); err != nil {
			closeServices()
			return err
		}
		return nil
	})
	return app
}
