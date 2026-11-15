package devices

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"

	"shellyficator/internal/domain"
	"shellyficator/internal/inventory"
	"shellyficator/internal/shelly"
	_ "shellyficator/migrations"
)

func testService(t *testing.T, handler http.HandlerFunc) (*Service, *httptest.Server) {
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
	s := New(inventory.New(app), shelly.NewClient(""), Options{Discovery: shelly.NetworkDiscovery{}})
	t.Cleanup(s.Close)
	return s, device
}

func refreshTestService(t *testing.T, handler http.HandlerFunc) (*Service, string) {
	t.Helper()
	s, device := testService(t, handler)
	record, err := s.store.ApproveReading(domain.Reading{
		Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Address: device.URL,
		Status: map[string]any{"input:0": map[string]any{"state": nil}, "switch:0": map[string]any{"output": false}},
		Config: map[string]any{"input:0": map[string]any{"type": "button", "enable": true}, "switch:0": map[string]any{"in_mode": "momentary"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, record.ID
}

func waitForJob(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("device job did not finish")
	}
}

func closeService(t *testing.T, service *Service) {
	t.Helper()
	closed := make(chan struct{})
	go func() { service.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("service did not cancel device work within one second")
	}
}

func TestCloseCancelsRefreshWithoutChangingHealthAndDrainsPool(t *testing.T) {
	started, cancelled := make(chan struct{}, 4), make(chan struct{}, 4)
	release := make(chan struct{})
	s, id := refreshTestService(t, func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-r.Context().Done():
			cancelled <- struct{}{}
		case <-release:
		}
	})
	t.Cleanup(func() { close(release) })
	before, err := s.store.Device(id)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < 8; i++ {
		if _, err := s.store.SaveReading(domain.Reading{
			Identity: domain.Identity{ID: fmt.Sprintf("device-%d", i), MAC: fmt.Sprintf("%012X", i), Gen: 4},
			Address:  before.Address, Status: map[string]any{}, Config: map[string]any{},
		}, false); err != nil {
			t.Fatal(err)
		}
	}
	done := s.Refresh(false)
	for i := 0; i < 4; i++ {
		waitForJob(t, started)
	}
	if s.Refresh(true) != done {
		t.Fatal("queued manual refresh did not share completion")
	}
	closed := make(chan struct{})
	go func() { s.Close(); close(closed) }()
	waitForJob(t, closed)
	waitForJob(t, done)
	for i := 0; i < 4; i++ {
		waitForJob(t, cancelled)
	}
	after, err := s.store.Device(id)
	if err != nil {
		t.Fatal(err)
	}
	if after.ConsecutiveFailures != before.ConsecutiveFailures || !after.StatusCurrent() || !after.ConfigCurrent() || after.LastChecked != before.LastChecked || after.BaselineID != before.BaselineID {
		t.Fatal("shutdown recorded an outage or changed the approved baseline")
	}
	snapshots, err := s.store.Snapshots(id)
	if err != nil || len(snapshots) != 1 || snapshots[0].ID != before.BaselineID {
		t.Fatalf("shutdown changed snapshot history: count=%d err=%v", len(snapshots), err)
	}
	if s.Refresh(true) != nil || s.Discover("", true) == nil {
		t.Fatal("closed service accepted new device work")
	}
	records, err := s.store.Devices()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.ConsecutiveFailures != 0 || !record.StatusCurrent() || !record.ConfigCurrent() {
			t.Fatalf("shutdown changed device health: %s", record.ID)
		}
	}
}

func TestInventoryKeepsCompletedPassAvailableDuringRefreshAndLoadsRelatedData(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	s, id := refreshTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			close(started)
			<-release
			_, _ = w.Write([]byte(`{"switch:0":{"output":true}}`))
		}
	})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	if err := s.store.SaveChannelNote(id, "switch:0", "Desk lamp"); err != nil {
		t.Fatal(err)
	}
	fastRead := make(chan struct{})
	fastDevice := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"fast-device","mac":"AABBCCDDEE01","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{"switch:0":{"output":true}}`))
			close(fastRead)
		}
	}))
	t.Cleanup(fastDevice.Close)
	if _, err := s.store.ApproveReading(domain.Reading{
		Identity: domain.Identity{ID: "fast-device", MAC: "AABBCCDDEE01", Gen: 4}, Address: fastDevice.URL,
		Status: map[string]any{"switch:0": map[string]any{"output": false}},
		Config: map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}},
	}); err != nil {
		t.Fatal(err)
	}
	done := s.Refresh(false)
	waitForJob(t, started)
	waitForJob(t, fastRead)
	result := make(chan struct {
		devices []domain.InventoryEntry
		err     error
	}, 1)
	go func() {
		devices, err := s.Inventory()
		result <- struct {
			devices []domain.InventoryEntry
			err     error
		}{devices, err}
	}()
	select {
	case snapshot := <-result:
		if snapshot.err != nil || len(snapshot.devices) != 2 {
			t.Fatalf("saved inventory snapshot: count=%d err=%v", len(snapshot.devices), snapshot.err)
		}
		for _, device := range snapshot.devices {
			if device.Device.Status["switch:0"].(map[string]any)["output"] != false {
				t.Fatal("inventory published part of an incomplete polling pass")
			}
		}
	case <-time.After(time.Second):
		t.Fatal("saved inventory waited for network reads")
	}
	close(release)
	waitForJob(t, done)
	snapshot, err := s.Inventory()
	if err != nil || len(snapshot) != 2 {
		t.Fatalf("inventory snapshot: count=%d err=%v", len(snapshot), err)
	}
	for _, device := range snapshot {
		if device.Device.Status["switch:0"].(map[string]any)["output"] != true || device.Baseline["switch:0"].(map[string]any)["in_mode"] != "momentary" || device.Device.ID == id && device.ChannelNotes["switch:0"] != "Desk lamp" {
			t.Fatal("inventory snapshot lost completed readings, baseline, or channel notes")
		}
	}
}

func TestCloseCancelsDiscoverySourcesAndRejectsDuplicateJob(t *testing.T) {
	s := New(nil, nil, Options{Discovery: shelly.NetworkDiscovery{}})
	t.Cleanup(s.Close)
	started, cancelled := make(chan struct{}, 2), make(chan struct{}, 2)
	query := func(ctx context.Context) ([]string, error) {
		started <- struct{}{}
		<-ctx.Done()
		cancelled <- struct{}{}
		return nil, ctx.Err()
	}
	s.announcements, s.neighbors = query, query
	if err := s.Discover("", true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		waitForJob(t, started)
	}
	if err := s.Discover("", true); err == nil || err.Error() != "Discovery is already running." {
		t.Fatalf("duplicate discovery: %v", err)
	}
	closeService(t, s)
	for i := 0; i < 2; i++ {
		waitForJob(t, cancelled)
	}
	if s.Discovery().Running || s.Discover("", true) == nil {
		t.Fatal("cancelled discovery stayed running or restarted")
	}
}

func TestCloseCancelsManualDiscoveryRPC(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	s, device := testService(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(cancelled)
	})
	if err := s.Discover(device.URL, false); err != nil {
		t.Fatal(err)
	}
	waitForJob(t, started)
	closeService(t, s)
	waitForJob(t, cancelled)
	records, err := s.store.Devices()
	if err != nil || len(records) != 0 || s.Discovery().Running {
		t.Fatalf("cancelled verification persisted data or kept running: count=%d err=%v", len(records), err)
	}
}

func TestCloseCanRaceJobStarts(t *testing.T) {
	s, _ := testService(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	s.announcements = func(ctx context.Context) ([]string, error) { return nil, ctx.Err() }
	s.neighbors = s.announcements
	var callers sync.WaitGroup
	for i := 0; i < 20; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			s.Refresh(true)
			_ = s.Discover("", true)
			s.Close()
		}()
	}
	callers.Wait()
	if s.Refresh(true) != nil || s.Discovery().Running {
		t.Fatal("shutdown allowed work to restart")
	}
}

// setDeviceField changes timestamps in adapter integration fixtures only.
func setDeviceField(t *testing.T, service *Service, id, field string, value any) {
	t.Helper()
	store := service.store.(*inventory.Store)
	record, err := store.App.FindRecordById("devices", id)
	if err != nil {
		t.Fatal(err)
	}
	record.Set(field, value)
	if err := store.App.Save(record); err != nil {
		t.Fatal(err)
	}
}
