package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"shellyficator/internal/accounts"
	"shellyficator/internal/accountstore"
	"shellyficator/internal/devices"
	"shellyficator/internal/domain"
	"shellyficator/internal/inventory"
	"shellyficator/internal/shelly"
	_ "shellyficator/migrations"
)

func refreshTestServer(t *testing.T, handler http.HandlerFunc) (*testServer, string) {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	device := httptest.NewUnstartedServer(handler)
	// Keep workstation readiness probes on 127.0.0.1 away from fake devices.
	_ = device.Listener.Close()
	listener, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Fatal(err)
	}
	device.Listener = listener
	device.Start()
	t.Cleanup(device.Close)
	store := inventory.New(app)
	record, err := store.ApproveReading(domain.Reading{
		Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Address: device.URL,
		Status: map[string]any{"input:0": map[string]any{"state": nil}, "switch:0": map[string]any{"output": false}},
		Config: map[string]any{"input:0": map[string]any{"type": "button", "enable": true}, "switch:0": map[string]any{"in_mode": "momentary"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	templates, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}
	client := shelly.NewClient("")
	service := devices.New(store, client, devices.Options{Discovery: shelly.NetworkDiscovery{}})
	t.Cleanup(func() {
		service.Close()
		_ = app.OnTerminate().Trigger(&core.TerminateEvent{App: app})
	})
	return &testServer{Server: &Server{watchStore: store, devices: service, accounts: accounts.New(accountstore.New(app)), templates: templates, loginAttempts: map[string]loginAttempt{}}, store: store, client: client, devices: service}, record.ID
}

func waitForRefresh(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh did not finish")
	}
}

func TestInventoryFiltersUseSavedReadingsWithoutContactingDevices(t *testing.T) {
	var requests atomic.Int32
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{"switch:0":{"output":true}}`))
		}
	})
	if err := s.store.Organization(id, "Kitchen", "light"); err != nil {
		t.Fatal(err)
	}
	before, err := s.record(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"", "q=test-device", "room=Kitchen", "tags=light", "group=room", "q=test-device&room=Kitchen&tags=light&group=room"} {
		t.Run(query, func(t *testing.T) {
			response := httptest.NewRecorder()
			event := &core.RequestEvent{App: s.store.App, Event: router.Event{Request: httptest.NewRequest("GET", "/inventory?"+query, nil), Response: response}}
			if err := s.table(event); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || !strings.Contains(renderedText(response.Body.String()), "test-device") || !strings.Contains(renderedText(response.Body.String()), "Off") {
				t.Fatalf("filter response lost saved device readings: %s", response.Body.String())
			}
			if requests.Load() != 0 {
				t.Fatalf("filter change contacted devices: %d requests", requests.Load())
			}
		})
	}
	after, err := s.record(id)
	if err != nil {
		t.Fatal(err)
	}
	if after.GetString("last_checked") != before.GetString("last_checked") || after.GetString("last_status_read") != before.GetString("last_status_read") || after.GetInt("consecutive_failures") != before.GetInt("consecutive_failures") {
		t.Fatal("filter changes modified device refresh state")
	}
	if done := s.devices.Refresh(false); done == nil {
		t.Fatal("filter changes consumed the automatic refresh interval")
	} else {
		waitForRefresh(t, done)
	}
}

func TestManualRefreshWaitsForSettingsAndReturnsUpdatedTable(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	s, _ := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{"input:0":{"state":null},"switch:0":{"output":true}}`))
		case "/rpc/Shelly.GetConfig":
			close(started)
			<-release
			_, _ = w.Write([]byte(`{"input:0":{"type":"button"},"switch:0":{"in_mode":"detached"}}`))
		}
	})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	response := httptest.NewRecorder()
	event := &core.RequestEvent{App: s.store.App, Event: router.Event{Request: httptest.NewRequest("GET", "/inventory/refresh?refresh=1", nil), Response: response}}
	result := make(chan error, 1)
	go func() { result <- s.refreshInventory(event) }()
	waitForRefresh(t, started)
	select {
	case err := <-result:
		t.Fatalf("manual response returned before configuration completed: %v", err)
	default:
	}
	// Another click/viewer shares the already running full read.
	done := s.devices.Refresh(true)
	close(release)
	waitForRefresh(t, done)
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("manual table response did not finish")
	}
	body := response.Body.String()
	if response.Code != 200 || !strings.Contains(renderedText(body), "Detached") || !strings.Contains(body, "Settings last read ·") || strings.Contains(body, "Settings unavailable") || strings.Contains(body, "Cached ·") {
		t.Fatalf("manual refresh returned stale or misleading table: %s", body)
	}
}

func TestAutomaticRefreshWaitsForCompletedResultsAndSharesThePass(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var requests atomic.Int32
	s, _ := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			requests.Add(1)
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			close(started)
			<-release
			_, _ = w.Write([]byte(`{"switch:0":{"output":true}}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	response := httptest.NewRecorder()
	event := &core.RequestEvent{App: s.store.App, Event: router.Event{Request: httptest.NewRequest("GET", "/inventory/refresh?group=room", nil), Response: response}}
	result := make(chan error, 1)
	go func() { result <- s.refreshInventory(event) }()
	waitForRefresh(t, started)
	select {
	case err := <-result:
		t.Fatalf("automatic response returned during an incomplete scan: %v", err)
	default:
	}
	done := s.devices.Refresh(false)
	// Filters can use the last saved readings while the shared poll is blocked.
	filteredResponse := httptest.NewRecorder()
	filtered := &core.RequestEvent{App: s.store.App, Event: router.Event{Request: httptest.NewRequest("GET", "/inventory?q=test-device&group=room", nil), Response: filteredResponse}}
	filteredResult := make(chan error, 1)
	go func() { filteredResult <- s.table(filtered) }()
	select {
	case err := <-filteredResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("filter query waited for device polling")
	}
	if requests.Load() != 1 || !strings.Contains(renderedText(filteredResponse.Body.String()), "Off") || filteredResponse.Header().Get("HX-Replace-Url") != "/?group=room&q=test-device" {
		t.Fatal("filter query lost saved readings or filters, or contacted devices")
	}
	// A disconnected viewer stops waiting without cancelling the shared poll.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelled := &core.RequestEvent{Event: router.Event{Request: httptest.NewRequest("GET", "/inventory/refresh", nil).WithContext(ctx), Response: httptest.NewRecorder()}}
	if err := s.refreshInventory(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled viewer did not stop waiting: %v", err)
	}
	select {
	case <-done:
		t.Fatal("cancelled viewer ended the shared poll")
	default:
	}
	close(release)
	waitForRefresh(t, done)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	currentOn := regexp.MustCompile(`class="[^"]*\btext-success\b[^"]*"[^>]*>\s*On(?:\s|<)`).MatchString(response.Body.String())
	if requests.Load() != 1 || !currentOn || response.Header().Get("HX-Replace-Url") != "/?group=room" {
		t.Fatal("automatic refresh duplicated the scan, lost filters, or returned old values")
	}
}
