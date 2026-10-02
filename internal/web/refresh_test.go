package web

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
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
	record, err := store.SaveReading(shelly.Reading{
		Identity: shelly.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Address: device.URL,
		Status: map[string]any{"input:0": map[string]any{"state": nil}, "switch:0": map[string]any{"output": false}},
		Config: map[string]any{"input:0": map[string]any{"type": "button", "enable": true}, "switch:0": map[string]any{"in_mode": "momentary"}},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkBaseline(record.Id); err != nil {
		t.Fatal(err)
	}
	functions := template.FuncMap{"get": func(r *core.Record, key string) string { return r.GetString(key) }, "flag": func(r *core.Record, key string) bool { return r.GetBool(key) }, "when": inventory.FormatTime, "value": inventory.Display, "tags": inventory.SplitTags}
	templates, err := template.New("app").Funcs(functions).ParseFS(assets, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	return &Server{store: store, client: shelly.NewClient(""), templates: templates}, record.Id
}

func waitForRefresh(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh did not finish")
	}
}

func TestConfigurationRefreshAndRecoveryPreserveHistory(t *testing.T) {
	var configs atomic.Int32
	var failConfig atomic.Bool
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{"input:0":{"state":null},"switch:0":{"output":true}}`))
		case "/rpc/Shelly.GetConfig":
			configs.Add(1)
			if failConfig.Load() {
				http.Error(w, "settings unavailable", http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{"input:0":{"type":"button","enable":true},"switch:0":{"in_mode":"detached"}}`))
		default:
			t.Errorf("unexpected device request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	record, _ := s.store.Device(id)
	baselineID := record.GetString("baseline")
	assertHistory := func() {
		t.Helper()
		record, err := s.store.Device(id)
		if err != nil {
			t.Fatal(err)
		}
		snapshots, err := s.store.Snapshots(id)
		if err != nil || len(snapshots) != 1 || snapshots[0].Id != baselineID || record.GetString("baseline") != baselineID {
			t.Fatalf("refresh changed capture history or baseline: snapshots=%v err=%v", snapshots, err)
		}
		if s.store.Baseline(record)["switch:0"].(map[string]any)["in_mode"] != "momentary" {
			t.Fatal("refresh modified approved baseline contents")
		}
	}

	s.refreshDevices(false)
	if configs.Load() != 0 {
		t.Fatal("status poll reread recent settings")
	}
	record, _ = s.store.Device(id)
	record.Set("last_captured", time.Now().UTC().Add(-6*time.Minute))
	if err := s.store.App.Save(record); err != nil {
		t.Fatal(err)
	}
	s.refreshDevices(false)
	record, _ = s.store.Device(id)
	view := tableDevice(record, s.store.Baseline(record), 1)
	if configs.Load() != 1 || view.SettingsWarning != "" || view.Inputs[0].Mode != "Detached" || !view.Inputs[0].Changed || time.Since(record.GetDateTime("last_captured").Time()) > time.Minute {
		t.Fatalf("periodic configuration read did not update settings: %+v", view)
	}
	assertHistory()

	lastRead := record.GetString("last_captured")
	failConfig.Store(true)
	s.refreshDevices(true)
	record, _ = s.store.Device(id)
	view = tableDevice(record, s.store.Baseline(record), 1)
	if !record.GetBool("reachable") || !record.GetBool("status_available") || record.GetBool("config_available") || record.GetString("last_captured") != lastRead || view.Inputs[0].Mode != "Detached" || view.Inputs[0].Output != "On" || view.Inputs[0].State != "Not reported" || view.SettingsWarning != "Settings unavailable · showing saved copy" {
		t.Fatalf("failed settings read lost saved settings or live status: %+v", view)
	}
	assertHistory()

	failConfig.Store(false)
	s.refreshDevices(false)
	record, _ = s.store.Device(id)
	if configs.Load() != 3 || !record.GetBool("config_available") || record.GetString("problem") != "" || tableDevice(record, nil, 0).SettingsWarning != "" {
		t.Fatal("next poll did not retry unavailable configuration and clear warning")
	}
	if err := s.store.Failed(id, "offline"); err != nil {
		t.Fatal(err)
	}
	s.refreshDevices(false)
	record, _ = s.store.Device(id)
	if configs.Load() != 4 || !record.GetBool("reachable") || !record.GetBool("config_available") {
		t.Fatal("reconnected device did not reread configuration")
	}
	assertHistory()
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
	done := s.refresh(true)
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

func TestManualRefreshDuringStatusPollQueuesConfigurationRead(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var statuses, configs atomic.Int32
	s, _ := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			if statuses.Add(1) == 1 {
				close(started)
				<-release
			}
			_, _ = w.Write([]byte(`{"switch:0":{"output":true}}`))
		case "/rpc/Shelly.GetConfig":
			configs.Add(1)
			_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"detached"}}`))
		}
	})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	done := s.refresh(false)
	waitForRefresh(t, started)
	if s.refresh(true) != done || s.refresh(true) != done {
		t.Fatal("concurrent requests did not share refresh completion")
	}
	close(release)
	waitForRefresh(t, done)
	if statuses.Load() != 2 || configs.Load() != 1 {
		t.Fatalf("manual configuration pass dropped or duplicated: status=%d config=%d", statuses.Load(), configs.Load())
	}
	if s.refresh(false) != nil {
		t.Fatal("shared refresh throttle did not suppress immediate polling")
	}
}
