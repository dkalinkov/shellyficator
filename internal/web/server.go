package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/dbx"
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
	discovery           Discovery
	refreshing          bool
	lastRefresh         time.Time
	refreshDone         chan struct{}
	refreshForced       bool
	refreshForcePending bool
	loginAttempts       map[string]loginAttempt
}

type loginAttempt struct {
	count int
	since time.Time
}

type Discovery struct {
	Running                 bool
	Total, Completed, Found int
	Target, Message         string
	Stage                   string
}
type Device struct {
	Record           *core.Record
	Name, Comparison string
	Differences      int
	Table            DeviceTable
}
type Channel struct {
	Key, Name, Notes string
	Values           []Metric
}
type Metric struct{ Label, Value string }
type Page struct {
	Title, Active, CSRF, Notice, Error, Query, Room string
	Version                                         string
	Devices                                         []Device
	Rooms, Tags                                     []string
	Total, Reachable, Changed, Unprotected          int
	Discovery                                       Discovery
	Networks                                        []shelly.Network
	Device                                          *core.Record
	Name                                            string
	Channels                                        []Channel
	Metrics                                         []Metric
	Differences                                     []inventory.Difference
	HasBaseline                                     bool
	Snapshots                                       []*core.Record
	Config                                          string
}

func Register(e *core.ServeEvent, store *inventory.Store, client *shelly.Client, version string) error {
	// The app's first-run form creates both accounts; the PocketBase installer is redundant.
	e.InstallerFunc = nil
	functions := template.FuncMap{"get": func(r *core.Record, key string) string { return r.GetString(key) }, "flag": func(r *core.Record, key string) bool { return r.GetBool(key) }, "when": inventory.FormatTime, "value": inventory.Display, "tags": inventory.SplitTags}
	templates, err := template.New("app").Funcs(functions).ParseFS(assets, "templates/*.html")
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
	e.Router.GET("/login", func(e *core.RequestEvent) error {
		count, err := store.App.CountRecords("operators")
		if err != nil {
			return err
		}
		if count == 0 {
			return e.Redirect(http.StatusSeeOther, "/setup")
		}
		return s.render(e, "login", Page{Title: "Sign in", CSRF: csrf(e)})
	})
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
	protected.GET("/devices/{id}/organization", s.organizationEditor)
	protected.POST("/devices/{id}/organization", s.organization).BindFunc(checkCSRF)
	protected.POST("/devices/{id}/metadata", s.metadata).BindFunc(checkCSRF)
	protected.POST("/devices/{id}/channel-notes", s.channelNote).BindFunc(checkCSRF)
	protected.POST("/devices/{id}/capture", s.capture).BindFunc(checkCSRF)
	protected.POST("/devices/{id}/baseline", s.baseline).BindFunc(checkCSRF)
	return nil
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

func (s *Server) render(e *core.RequestEvent, name string, page Page) error {
	page.Version = s.version
	var buffer bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buffer, name, page); err != nil {
		return err
	}
	return e.HTML(200, buffer.String())
}

func (s *Server) list(e *core.RequestEvent) (Page, error) {
	return s.listFiltered(e, e.Request.URL.Query().Get("q"), e.Request.URL.Query().Get("room"))
}

func (s *Server) listFiltered(e *core.RequestEvent, query, roomFilter string) (Page, error) {
	records, err := s.store.Devices()
	if err != nil {
		return Page{}, err
	}
	page := Page{Title: "Devices", Active: "devices", CSRF: csrf(e), Query: strings.TrimSpace(query), Room: roomFilter, Notice: e.Request.URL.Query().Get("notice")}
	rooms := map[string]bool{}
	page.Total = len(records)
	for _, record := range records {
		name := record.GetString("name")
		if name == "" {
			name = record.GetString("device_id")
		}
		device := Device{Record: record, Name: name, Comparison: "No baseline"}
		if record.GetBool("reachable") {
			page.Reachable++
		}
		if !record.GetBool("auth_enabled") {
			page.Unprotected++
		}
		baseline := s.store.Baseline(record)
		if baseline != nil {
			device.Comparison = "Not checked"
			if record.GetBool("reachable") && record.GetBool("config_available") {
				device.Differences = len(inventory.Differences(baseline, inventory.Object(record, "config")))
				device.Comparison = "Matches baseline"
				if device.Differences > 0 {
					device.Comparison = "Configuration changed"
					page.Changed++
				}
			}
		}
		room := record.GetString("room")
		if room != "" {
			rooms[room] = true
		}
		haystack := strings.ToLower(strings.Join([]string{name, record.GetString("device_id"), record.GetString("address"), room, record.GetString("tags"), record.GetString("notes")}, " "))
		if page.Query != "" && !strings.Contains(haystack, strings.ToLower(page.Query)) {
			continue
		}
		if page.Room != "" && page.Room != room {
			continue
		}
		device.Table = tableDevice(record, baseline, device.Differences)
		notes := s.store.ChannelNotes(record.Id)
		for i := range device.Table.Inputs {
			input := &device.Table.Inputs[i]
			input.Note = previewText(notes[input.Key], 320)
			input.RelayNote = previewText(notes[input.RelayKey], 320)
		}
		page.Devices = append(page.Devices, device)
	}
	for room := range rooms {
		page.Rooms = append(page.Rooms, room)
	}
	// Keep an active filter available even after its last device moves rooms.
	if page.Room != "" && !rooms[page.Room] {
		page.Rooms = append(page.Rooms, page.Room)
	}
	sort.Strings(page.Rooms)
	s.mu.Lock()
	page.Discovery = s.discovery
	s.mu.Unlock()
	return page, nil
}

func (s *Server) dashboard(e *core.RequestEvent) error {
	page, err := s.list(e)
	if err != nil {
		return err
	}
	page.Networks, _ = shelly.LocalNetworks()
	return s.render(e, "dashboard", page)
}
func (s *Server) table(e *core.RequestEvent) error {
	force := e.Request.URL.Query().Get("refresh") == "1"
	done := s.refresh(force)
	if force && done != nil {
		select {
		case <-done:
		case <-e.Request.Context().Done():
			return e.Request.Context().Err()
		}
	}
	page, err := s.list(e)
	if err != nil {
		return err
	}
	return s.render(e, "inventory", page)
}
func (s *Server) progress(e *core.RequestEvent) error {
	s.mu.Lock()
	job := s.discovery
	s.mu.Unlock()
	return s.render(e, "discovery", Page{Discovery: job})
}

func (s *Server) refresh(force bool) <-chan struct{} {
	s.mu.Lock()
	if s.refreshing {
		// A manual request during status polling gets a configuration pass too.
		if force && !s.refreshForced {
			s.refreshForcePending = true
		}
		done := s.refreshDone
		s.mu.Unlock()
		return done
	}
	if !force && time.Since(s.lastRefresh) < 30*time.Second {
		s.mu.Unlock()
		return nil
	}
	s.refreshing = true
	s.refreshForced = force
	s.lastRefresh = time.Now()
	s.refreshDone = make(chan struct{})
	done := s.refreshDone
	s.mu.Unlock()
	go func() {
		for {
			s.refreshDevices(force)
			s.mu.Lock()
			if s.refreshForcePending {
				s.refreshForcePending = false
				s.refreshForced = true
				force = true
				s.lastRefresh = time.Now()
				s.mu.Unlock()
				continue
			}
			s.refreshing = false
			s.refreshForced = false
			close(done)
			s.mu.Unlock()
			return
		}
	}()
	return done
}

func (s *Server) refreshDevices(force bool) {
	records, err := s.store.Devices()
	if err != nil {
		s.store.App.Logger().Error("Could not load devices for refresh", "error", err)
		return
	}
	limit := make(chan struct{}, 4)
	var group sync.WaitGroup
	for _, record := range records {
		limit <- struct{}{}
		group.Add(1)
		go func(record *core.Record) {
			defer func() { <-limit; group.Done() }()
			lastRead := record.GetDateTime("last_captured").Time()
			readConfig := force || !record.GetBool("reachable") || !record.GetBool("config_available") || time.Since(lastRead) >= 5*time.Minute
			timeout := 8 * time.Second
			if readConfig {
				timeout = 12 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			reading, err := s.client.Read(ctx, record.GetString("address"), record.GetString("mac"), readConfig)
			if err != nil {
				problem := "Device could not be reached. Last successful readings are preserved."
				if errors.Is(err, shelly.ErrIdentity) {
					problem = "Address now belongs to a different device. Rediscover this device."
				}
				_ = s.store.Failed(record.Id, problem)
				return
			}
			if _, err := s.store.SaveReading(reading, false); err != nil {
				s.store.App.Logger().Error("Could not save device reading", "device", record.Id)
			}
		}(record)
	}
	group.Wait()
}

func (s *Server) discover(e *core.RequestEvent) error {
	target := strings.TrimSpace(e.Request.FormValue("target"))
	automatic := e.Request.FormValue("mode") == "auto"
	var addresses []string
	if !automatic {
		var err error
		addresses, err = shelly.Targets(target)
		if err != nil {
			return s.redirect(e, "/", err.Error())
		}
	} else {
		target = "network announcements"
	}
	s.mu.Lock()
	if s.discovery.Running {
		s.mu.Unlock()
		return s.redirect(e, "/", "Discovery is already running.")
	}
	s.discovery = Discovery{Running: true, Total: len(addresses), Target: target, Stage: "Scanning addresses"}
	if automatic {
		s.discovery.Stage = "Finding devices"
	}
	s.mu.Unlock()
	go s.runDiscovery(automatic, addresses)
	return s.redirect(e, "/", "")
}

func (s *Server) runDiscovery(automatic bool, addresses []string) {
	announcementFailed := false
	if automatic {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		type neighborResult struct {
			addresses []string
			err       error
		}
		neighborCh := make(chan neighborResult, 1)
		go func() {
			if s.neighbors == nil {
				neighborCh <- neighborResult{}
				return
			}
			addresses, err := s.neighbors(ctx)
			neighborCh <- neighborResult{addresses, err}
		}()
		announced, err := s.announcements(ctx)
		neighbors := <-neighborCh
		cancel()
		announcementFailed = err != nil
		addresses = shelly.DiscoveryAddresses(append(announced, neighbors.addresses...))
		if s.store != nil {
			s.store.App.Logger().Info("Discovery candidates", "announced", len(announced), "neighbors", len(neighbors.addresses), "candidates", len(addresses), "mdns_error", err, "neighbor_error", neighbors.err)
		}
		s.mu.Lock()
		s.discovery.Total = len(addresses)
		s.discovery.Stage = "Verifying devices"
		s.mu.Unlock()
	}
	limit := make(chan struct{}, 4)
	var group sync.WaitGroup
	foundMACs := map[string]bool{}
	for _, address := range addresses {
		limit <- struct{}{}
		group.Add(1)
		go func(address string) {
			defer func() { <-limit; group.Done() }()
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			reading, err := s.client.Discover(ctx, address)
			found := false
			if err == nil {
				_, err = s.store.SaveReading(reading, true)
				found = err == nil
			}
			s.mu.Lock()
			s.discovery.Completed++
			if found && !foundMACs[reading.Identity.MAC] {
				foundMACs[reading.Identity.MAC] = true
				s.discovery.Found++
			}
			if !automatic && len(addresses) == 1 && err != nil {
				s.discovery.Message = shelly.DiscoveryError(err)
			}
			s.mu.Unlock()
		}(address)
		if !automatic {
			time.Sleep(50 * time.Millisecond)
		}
	}
	group.Wait()
	s.mu.Lock()
	s.discovery.Running = false
	if s.discovery.Message == "" {
		s.discovery.Message = fmt.Sprintf("Devices found: %d.", s.discovery.Found)
		if automatic && s.discovery.Found == 0 {
			s.discovery.Message = fmt.Sprintf("No supported devices responded at %d discovered addresses.", len(addresses))
			if len(addresses) == 0 {
				s.discovery.Message = "No Shelly announcements received. Check multicast access."
				if announcementFailed {
					s.discovery.Message = "mDNS query failed. Check multicast access."
				}
			}
		}
		if automatic && announcementFailed && s.discovery.Found > 0 {
			s.discovery.Message += " Some mDNS queries failed."
		}
	}
	s.mu.Unlock()
}

func (s *Server) detail(e *core.RequestEvent) error {
	record, err := s.store.Device(e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Device not found", nil)
	}
	page := Page{Title: "Device", Active: "devices", CSRF: csrf(e), Device: record, Notice: e.Request.URL.Query().Get("notice"), Name: record.GetString("name")}
	page.Rooms, page.Tags, err = s.store.OrganizationOptions()
	if err != nil {
		return err
	}
	if page.Name == "" {
		page.Name = record.GetString("device_id")
	}
	status, config := inventory.Object(record, "status"), inventory.Object(record, "config")
	keys := map[string]bool{}
	for key := range status {
		if strings.Contains(key, ":") {
			keys[key] = true
		}
	}
	for key := range config {
		if strings.Contains(key, ":") {
			keys[key] = true
		}
	}
	var ordered []string
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	notes := s.store.ChannelNotes(record.Id)
	for _, key := range ordered {
		channel := Channel{Key: key, Name: key, Notes: notes[key]}
		settings, _ := config[key].(map[string]any)
		reading, _ := status[key].(map[string]any)
		if name, ok := settings["name"].(string); ok && name != "" {
			channel.Name = name
		}
		for _, field := range []struct{ key, label, unit string }{{"output", "Output", ""}, {"state", "Physical input", ""}, {"current_pos", "Position", "%"}} {
			if value, exists := reading[field.key]; exists {
				channel.Values = append(channel.Values, Metric{field.label, inventory.Display(value) + field.unit})
			}
		}
		if temperature, ok := reading["temperature"].(map[string]any); ok {
			channel.Values = append(channel.Values, Metric{"Temperature", inventory.Display(temperature["tC"]) + " °C"})
		}
		for _, field := range []struct{ key, label string }{{"type", "Physical input type"}, {"in_mode", "Input behaviour"}, {"initial_state", "After power loss"}, {"in_locked", "Input locked"}, {"enable", "Enabled"}} {
			if value, exists := settings[field.key]; exists {
				channel.Values = append(channel.Values, Metric{field.label, inventory.Display(value)})
			}
		}
		if errs, ok := reading["errors"]; ok {
			channel.Values = append(channel.Values, Metric{"Reported errors", inventory.Display(errs)})
		}
		page.Channels = append(page.Channels, channel)
	}
	if wifi, ok := status["wifi"].(map[string]any); ok {
		page.Metrics = append(page.Metrics, Metric{"Wi-Fi", inventory.Display(wifi["status"])}, Metric{"Signal", inventory.Display(wifi["rssi"]) + " dBm"}, Metric{"Network", inventory.Display(wifi["ssid"])})
	}
	if sys, ok := status["sys"].(map[string]any); ok {
		page.Metrics = append(page.Metrics, Metric{"Uptime", inventory.Display(sys["uptime"]) + " s"}, Metric{"Restart required", inventory.Display(sys["restart_required"])})
	}
	for _, key := range []string{"cloud", "mqtt"} {
		if component, ok := status[key].(map[string]any); ok {
			page.Metrics = append(page.Metrics, Metric{strings.ToUpper(key) + " connected", inventory.Display(component["connected"])})
		}
	}
	baseline := s.store.Baseline(record)
	page.HasBaseline = baseline != nil
	if page.HasBaseline && record.GetBool("reachable") && record.GetBool("config_available") {
		page.Differences = inventory.Differences(baseline, config)
	}
	page.Snapshots, err = s.store.Snapshots(record.Id)
	if err != nil {
		return err
	}
	return s.render(e, "detail", page)
}

func (s *Server) redirect(e *core.RequestEvent, path, notice string) error {
	if notice != "" {
		path += "?notice=" + url.QueryEscape(notice)
	}
	return e.Redirect(http.StatusSeeOther, path)
}

func (s *Server) metadata(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if err := s.store.Metadata(id, e.Request.FormValue("room"), e.Request.FormValue("tags"), e.Request.FormValue("notes")); err != nil {
		return e.BadRequestError("Could not save device information", nil)
	}
	return s.redirect(e, "/devices/"+id, "Device info saved.")
}

func (s *Server) organizationEditor(e *core.RequestEvent) error {
	record, err := s.store.Device(e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Device not found", nil)
	}
	page := Page{Device: record, CSRF: csrf(e), Name: record.GetString("name")}
	if page.Name == "" {
		page.Name = record.GetString("device_id")
	}
	page.Rooms, page.Tags, err = s.store.OrganizationOptions()
	if err != nil {
		return err
	}
	return s.render(e, "organization-editor", page)
}

func (s *Server) organization(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if err := s.store.Organization(id, e.Request.FormValue("room"), e.Request.FormValue("tags")); err != nil {
		return e.BadRequestError("Could not save room & tags. Room names allow 100 characters; tags allow 500 in total.", nil)
	}
	if e.Request.Header.Get("HX-Request") != "true" {
		return s.redirect(e, "/", "Room & tags saved.")
	}
	page, err := s.listFiltered(e, e.Request.FormValue("filter_q"), e.Request.FormValue("filter_room"))
	if err != nil {
		return err
	}
	return s.render(e, "organization-saved", page)
}

func (s *Server) channelNote(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if err := s.store.SaveChannelNote(id, e.Request.FormValue("component"), e.Request.FormValue("notes")); err != nil {
		return e.BadRequestError("Could not save channel note", nil)
	}
	return s.redirect(e, "/devices/"+id, "Note saved.")
}

func (s *Server) captureReading(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := s.store.Device(id)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(e.Request.Context(), 12*time.Second)
	defer cancel()
	reading, err := s.client.Read(ctx, record.GetString("address"), record.GetString("mac"), true)
	if err != nil {
		_ = s.store.Failed(id, "Configuration could not be captured. Check device identity and connectivity.")
		return err
	}
	if reading.Config == nil {
		_, _ = s.store.SaveReading(reading, true)
		return errors.New("configuration could not be read; check connectivity and shared device credentials")
	}
	_, err = s.store.SaveReading(reading, true)
	return err
}

func (s *Server) capture(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if err := s.captureReading(e); err != nil {
		return s.redirect(e, "/devices/"+id, "Capture failed. Check device access.")
	}
	return s.redirect(e, "/devices/"+id, "Configuration captured.")
}

func (s *Server) baseline(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if err := s.captureReading(e); err != nil {
		return s.redirect(e, "/devices/"+id, "Baseline unchanged: config read failed.")
	}
	if err := s.store.MarkBaseline(id); err != nil {
		return e.BadRequestError("Could not approve baseline", nil)
	}
	return s.redirect(e, "/devices/"+id, "Baseline updated.")
}
