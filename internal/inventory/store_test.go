package inventory

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
	_ "shellyficator/migrations"
)

func testStore(t *testing.T) *Store {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	return New(app)
}

func TestRediscoveryPreservesMetadataAndBaseline(t *testing.T) {
	store := testStore(t)
	reading := domain.Reading{Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Address: "http://192.168.1.2", Status: map[string]any{"switch:0": map[string]any{"output": false}}, Config: map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}}}
	device, err := store.ApproveReading(reading)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Metadata(device.ID, "Bedroom", "lighting", "Button beside door"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveChannelNote(device.ID, "switch:0", "Ceiling lights"); err != nil {
		t.Fatal(err)
	}
	reading.Address = "http://192.168.1.99"
	reading.Config = map[string]any{"switch:0": map[string]any{"in_mode": "detached"}}
	device, err = store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	if device.Notes != "Button beside door" || device.Room != "Bedroom" || testChannelNotes(t, store, device.ID)["switch:0"] != "Ceiling lights" {
		t.Fatal("rediscovery discarded local information")
	}
	devices, _ := store.Devices()
	if len(devices) != 1 {
		t.Fatal("IP change created another device")
	}
	if device.Address != reading.Address || !reflect.DeepEqual(device.Config, reading.Config) ||
		testBaseline(t, store, device)["switch:0"].(map[string]any)["in_mode"] != "momentary" {
		t.Fatal("rediscovery lost the new reading or replaced the approved baseline")
	}
}

func TestApprovalSelectsItsOwnCaptureEvenWithNewerHistory(t *testing.T) {
	store := testStore(t)
	reading := domain.Reading{Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Config: map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}}}
	device, err := store.SaveReading(reading, true)
	if err != nil {
		t.Fatal(err)
	}
	// Snapshot ordering must not decide which fresh read an approval selects.
	snapshots, err := store.Snapshots(device.ID)
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("expected one manual capture: %v, %v", snapshots, err)
	}
	snapshotRecord, err := store.App.FindRecordById("snapshots", snapshots[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshotRecord.Set("captured", time.Now().UTC().Add(time.Hour))
	if err := store.App.Save(snapshotRecord); err != nil {
		t.Fatal(err)
	}
	reading.Config = map[string]any{"switch:0": map[string]any{"in_mode": "detached"}}
	device, err = store.ApproveReading(reading)
	if err != nil {
		t.Fatal(err)
	}
	if device.BaselineID == snapshots[0].ID || !reflect.DeepEqual(testBaseline(t, store, device), reading.Config) {
		t.Fatal("approval selected another capture instead of its own fresh read")
	}
	reading.Config = nil
	if _, err := store.ApproveReading(reading); err == nil {
		t.Fatal("approval accepted an unavailable configuration")
	}
	after, err := store.Device(device.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err = store.Snapshots(device.ID)
	if err != nil || len(snapshots) != 2 || after.BaselineID != device.BaselineID || !reflect.DeepEqual(after.Config, device.Config) {
		t.Fatal("failed approval changed the baseline, readings, or capture history")
	}
}

func TestApprovalRollsBackCaptureWhenBaselineSaveFails(t *testing.T) {
	store := testStore(t)
	reading := domain.Reading{Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Config: map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}}}
	before, err := store.ApproveReading(reading)
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("baseline save failed")
	store.App.OnRecordUpdate("devices").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetString("baseline") != before.BaselineID {
			return wantErr
		}
		return e.Next()
	})
	reading.Config = map[string]any{"switch:0": map[string]any{"in_mode": "detached"}}
	if _, err := store.ApproveReading(reading); !errors.Is(err, wantErr) {
		t.Fatalf("approval did not return the baseline save error: %v", err)
	}
	after, err := store.Device(before.ID)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := store.Snapshots(before.ID)
	if err != nil || len(snapshots) != 1 || after.BaselineID != before.BaselineID || !reflect.DeepEqual(after.Config, before.Config) {
		t.Fatal("failed baseline save partially committed its capture or reading")
	}
}

func TestConcurrentFirstChannelNoteSavesUpdateOneRecord(t *testing.T) {
	store := testStore(t)
	device, err := store.SaveReading(domain.Reading{Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Config: map[string]any{"input:0": map[string]any{"type": "button"}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errors := make(chan error, 8)
	var group sync.WaitGroup
	for i := 0; i < cap(errors); i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			<-start
			errors <- store.SaveChannelNote(device.ID, "input:0", fmt.Sprintf("Note %d", i))
		}(i)
	}
	close(start)
	group.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent save failed: %v", err)
		}
	}
	if count, err := store.App.CountRecords("channel_notes"); err != nil || count != 1 || testChannelNotes(t, store, device.ID)["input:0"] == "" {
		t.Fatal("concurrent saves did not retain exactly one channel note")
	}
}

type lookupFailureApp struct {
	core.App
	err error
}

func (app lookupFailureApp) RunInTransaction(fn func(core.App) error) error {
	return app.App.RunInTransaction(func(tx core.App) error {
		return fn(lookupFailureApp{App: tx, err: app.err})
	})
}

func (app lookupFailureApp) FindFirstRecordByFilter(any, string, ...dbx.Params) (*core.Record, error) {
	return nil, app.err
}

func TestLookupFailuresAreNotTreatedAsMissingRecords(t *testing.T) {
	store := testStore(t)
	reading := domain.Reading{Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Config: map[string]any{"input:0": map[string]any{"type": "button"}}}
	device, err := store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("record lookup failed")
	failingStore := New(lookupFailureApp{App: store.App, err: wantErr})
	if _, err := failingStore.SaveReading(reading, true); !errors.Is(err, wantErr) {
		t.Fatalf("reading save hid the lookup error: %v", err)
	}
	if err := failingStore.SaveChannelNote(device.ID, "input:0", "Unsaved"); !errors.Is(err, wantErr) {
		t.Fatalf("channel note save hid the lookup error: %v", err)
	}
	for collection, wantCount := range map[string]int64{"devices": 1, "snapshots": 0, "channel_notes": 0} {
		if count, err := store.App.CountRecords(collection); err != nil || count != wantCount {
			t.Fatalf("lookup failure changed %s records: count=%d err=%v", collection, count, err)
		}
	}
}

func TestSnapshotQueriesReportStorageFailure(t *testing.T) {
	store := testStore(t)
	if _, err := store.App.DB().NewQuery("DROP TABLE snapshots").Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Snapshots("device"); err == nil {
		t.Fatal("unavailable storage appeared to be empty capture history")
	}
	if _, err := store.Baseline(domain.Device{ID: "device", BaselineID: "saved"}); err == nil {
		t.Fatal("unavailable storage appeared to be a missing baseline")
	}
}

func TestMissingDeviceAndInvalidBaselineRemainDistinct(t *testing.T) {
	store := testStore(t)
	if _, err := store.Device("missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing device did not return the domain error: %v", err)
	}
	reading := domain.Reading{Identity: domain.Identity{ID: "first", MAC: "AABBCCDDEEFF", Gen: 4}, Config: map[string]any{"switch:0": map[string]any{"enable": false}}}
	first, err := store.ApproveReading(reading)
	if err != nil {
		t.Fatal(err)
	}
	reading.Identity.ID, reading.Identity.MAC = "second", "AABBCCDDEEAA"
	second, err := store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, baselineID := range []string{"", "missing", first.BaselineID} {
		second.BaselineID = baselineID
		baseline, err := store.Baseline(second)
		if err != nil || baseline != nil {
			t.Fatalf("missing or cross-device baseline was accepted: id=%q baseline=%v err=%v", baselineID, baseline, err)
		}
	}
}

func TestReadingMapsPreservePresenceAndDoNotCreateAutomaticSnapshots(t *testing.T) {
	store := testStore(t)
	reading := domain.Reading{
		Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4},
		Status:   map[string]any{"input:0": map[string]any{"state": nil, "enable": false, "voltage": float64(0)}},
		Config:   map[string]any{"switch:0": map[string]any{"enable": false, "power_limit": float64(0), "name": nil}},
	}
	device, err := store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Device(device.ID)
	if err != nil || !reflect.DeepEqual(loaded.Status, reading.Status) || !reflect.DeepEqual(loaded.Config, reading.Config) {
		t.Fatalf("reading maps lost null, false, or zero: %+v, %v", loaded, err)
	}
	reading.Status, reading.Config, reading.ConfigAttempted = nil, nil, true
	updated, err := store.SaveReading(reading, false)
	if err != nil || updated.StatusAvailable || updated.ConfigAvailable || !reflect.DeepEqual(updated.Status, loaded.Status) || !reflect.DeepEqual(updated.Config, loaded.Config) || !updated.LastStatusRead.Equal(loaded.LastStatusRead) || !updated.LastCaptured.Equal(loaded.LastCaptured) {
		t.Fatalf("unavailable readings changed saved values or timestamps: %+v, %v", updated, err)
	}
	if count, err := store.App.CountRecords("snapshots"); err != nil || count != 0 || updated.BaselineID != "" {
		t.Fatalf("automatic readings created history: count=%d baseline=%q err=%v", count, updated.BaselineID, err)
	}
}

func testBaseline(t *testing.T, store *Store, device domain.Device) map[string]any {
	t.Helper()
	baseline, err := store.Baseline(device)
	if err != nil {
		t.Fatal(err)
	}
	return baseline
}

func testChannelNotes(t *testing.T, store *Store, id string) map[string]string {
	t.Helper()
	notes, err := store.ChannelNotes(id)
	if err != nil {
		t.Fatal(err)
	}
	return notes
}

func TestDiscoveryNetworks(t *testing.T) {
	store := testStore(t)
	for _, network := range []string{"192.168.1.77/24", "192.168.1.0/24", "10.0.5.0/24"} {
		if err := store.RememberNetwork(network); err != nil {
			t.Fatalf("RememberNetwork(%q): %v", network, err)
		}
	}
	if err := store.RememberNetwork("8.8.8.0/24"); err == nil {
		t.Error("RememberNetwork accepted a public network")
	}
	networks, err := store.DiscoveryNetworks()
	if err != nil || strings.Join(networks, " ") != "10.0.5.0/24 192.168.1.0/24" {
		t.Fatalf("networks were not normalized, deduplicated, and sorted: %v %v", networks, err)
	}
	for _, network := range []string{"192.168.10.0/24", "192.168.9.0/24"} {
		if err := store.RememberNetwork(network); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RememberNetwork("172.16.1.0/24"); !errors.Is(err, domain.ErrNetworkLimit) {
		t.Fatalf("fifth network: got %v, want the network limit", err)
	}
	if err := store.RememberNetwork("10.0.5.0/24"); err != nil {
		t.Fatalf("remembering a known network at the limit failed: %v", err)
	}
	networks, _ = store.DiscoveryNetworks()
	if strings.Join(networks, " ") != "10.0.5.0/24 192.168.1.0/24 192.168.9.0/24 192.168.10.0/24" {
		t.Fatalf("networks are not in address order: %v", networks)
	}
	if err := store.ForgetNetwork("192.168.1.9/24"); err != nil {
		t.Fatal(err)
	}
	if err := store.ForgetNetwork("192.168.77.0/24"); err != nil {
		t.Fatalf("forgetting an unknown network failed: %v", err)
	}
	networks, _ = store.DiscoveryNetworks()
	if strings.Join(networks, " ") != "10.0.5.0/24 192.168.9.0/24 192.168.10.0/24" {
		t.Fatalf("network was not forgotten: %v", networks)
	}
}
