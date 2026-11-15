package web

import (
	"io/fs"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

func (s *Server) registerRoutes(r *router.Router[*core.RequestEvent], static fs.FS) {
	r.GET("/assets/{path...}", apis.Static(static, false))
	r.GET("/healthz", func(e *core.RequestEvent) error { return e.String(200, "ok") })
	r.GET("/favicon.ico", func(e *core.RequestEvent) error { return e.NoContent(204) })
	r.GET("/login", s.loginPage)
	r.GET("/setup", s.setupPage)
	if s.oidc != nil {
		r.GET("/auth/oidc/callback", s.oidcCallback)
	}

	publicWrites := r.Group("").BindFunc(checkCSRF)
	publicWrites.POST("/setup", s.setup)
	publicWrites.POST("/login", s.login)
	if s.oidc != nil {
		publicWrites.POST("/login/authelia", s.oidcLogin)
	}

	protected := r.Group("").BindFunc(s.requireLogin)
	protected.GET("/{$}", s.dashboard)
	protected.GET("/inventory", s.table)
	protected.GET("/inventory/refresh", s.refreshInventory)
	protected.GET("/discovery/progress", s.progress)

	writes := protected.Group("").BindFunc(checkCSRF)
	writes.POST("/logout", s.logout)
	writes.POST("/discover", s.discover)

	devices := protected.Group("/devices/{id}")
	devices.GET("", s.detail)
	devices.GET("/organization", s.organizationEditor)

	deviceWrites := devices.Group("").BindFunc(checkCSRF)
	deviceWrites.POST("/refresh", s.refreshDevice)
	deviceWrites.POST("/firmware/check", s.checkFirmware)
	deviceWrites.POST("/organization", s.organization)
	deviceWrites.POST("/metadata", s.metadata)
	deviceWrites.POST("/channel-notes", s.channelNote)
	deviceWrites.POST("/capture", s.capture)
	deviceWrites.POST("/baseline", s.baseline)
}
