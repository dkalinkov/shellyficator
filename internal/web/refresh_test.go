package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"shellyficator/internal/devices"
	"shellyficator/internal/inventory"
	"shellyficator/internal/shelly"
	_ "shellyficator/migrations"
)

func refreshTestServer(t *testing.T, handler http.HandlerFunc) (*Server, string) {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	device := httptest.NewServer(handler)
	t.Cleanup(device.Close)
	store := inventory.New(app)
	record, err := store.ApproveReading(shelly.Reading{
		Identity: shelly.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Address: device.URL,
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
	service := devices.New(store, client)
	t.Cleanup(func() {
		service.Close()
		_ = app.OnTerminate().Trigger(&core.TerminateEvent{App: app})
	})
	return &Server{store: store, client: client, devices: service, templates: templates}, record.Id
}

func waitForRefresh(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh did not finish")
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
	event := &core.RequestEvent{App: s.store.App, Event: router.Event{Request: httptest.NewRequest("GET", "/inventory?refresh=1", nil), Response: response}}
	result := make(chan error, 1)
	go func() { result <- s.table(event) }()
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
	if response.Code != 200 || !strings.Contains(body, ">Detached</span>") || !strings.Contains(body, "Settings last read ·") || strings.Contains(body, "Settings unavailable") || strings.Contains(body, "Cached ·") {
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
	event := &core.RequestEvent{App: s.store.App, Event: router.Event{Request: httptest.NewRequest("GET", "/inventory?group=room", nil), Response: response}}
	result := make(chan error, 1)
	go func() { result <- s.table(event) }()
	waitForRefresh(t, started)
	select {
	case err := <-result:
		t.Fatalf("automatic response returned during an incomplete scan: %v", err)
	default:
	}
	done := s.devices.Refresh(false)
	// A disconnected viewer stops waiting without cancelling the shared poll.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelled := &core.RequestEvent{Event: router.Event{Request: httptest.NewRequest("GET", "/inventory", nil).WithContext(ctx), Response: httptest.NewRecorder()}}
	if err := s.table(cancelled); !errors.Is(err, context.Canceled) {
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
	if requests.Load() != 1 || !strings.Contains(response.Body.String(), `text-success">On`) || response.Header().Get("HX-Replace-Url") != "/?group=room" {
		t.Fatal("automatic refresh duplicated the scan, lost filters, or returned old values")
	}
}
