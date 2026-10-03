package inventory

import (
	"errors"
	"fmt"
	"reflect"
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
	device, err = store.SaveReading(reading, true)
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
	diff := domain.Differences(testBaseline(t, store, device), device.Config)
	if len(diff) != 1 || !diff[0].After.Present || diff[0].After.Value != "detached" {
		t.Fatalf("baseline not preserved or drift missed: %+v", diff)
	}
	if err := store.Failed(device.ID, "unreachable", false); err != nil {
		t.Fatal(err)
	}
	device, _ = store.Device(device.ID)
	if testBaseline(t, store, device) == nil || device.Config == nil || device.ConfigAvailable {
		t.Fatal("failed scan discarded history or presented it as current")
	}
	reading.Config = nil
	device, err = store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	if !device.Reachable || !device.StatusAvailable || device.ConfigAvailable {
		t.Fatal("status recovery presented the old configuration as newly checked")
	}
	if _, err := store.ApproveReading(reading); err == nil {
		t.Fatal("approved a baseline without an available configuration capture")
	}
}

func TestDiffDistinguishesMissingFromNullAndFalse(t *testing.T) {
	diff := domain.Differences(map[string]any{"input": map[string]any{"state": nil, "enable": false}}, map[string]any{"input": map[string]any{"enable": true}})
	if len(diff) != 2 || !diff[1].Before.Present || diff[1].Before.Value != nil || diff[1].After.Present {
		t.Fatalf("unexpected diff: %+v", diff)
	}
}

func TestApprovalUsesItsOwnCaptureAndOtherReadsPreserveIt(t *testing.T) {
	store := testStore(t)
	reading := domain.Reading{Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Config: map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}}}
	device, err := store.SaveReading(reading, true)
	if err != nil {
		t.Fatal(err)
	}
	// Snapshot ordering must not decide which fresh read an approval selects.
	snapshots, err := store.Snapshots(device.ID)
	if err != nil {
		t.Fatal(err)
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
	baselineID := device.BaselineID
	if baselineID == snapshots[0].ID || len(domain.Differences(testBaseline(t, store, device), reading.Config)) != 0 {
		t.Fatal("approval selected another capture instead of its own fresh read")
	}
	reading.Config = map[string]any{"switch:0": map[string]any{"in_mode": "follow"}}
	for _, capture := range []bool{false, true} {
		device, err = store.SaveReading(reading, capture)
		if err != nil || device.BaselineID != baselineID || testBaseline(t, store, device)["switch:0"].(map[string]any)["in_mode"] != "detached" {
			t.Fatal("a later read or capture changed the approved snapshot")
		}
	}
	before, err := store.Device(device.ID)
	if err != nil {
		t.Fatal(err)
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
	if err != nil || len(snapshots) != 3 || after.BaselineID != baselineID || !reflect.DeepEqual(after.Config, before.Config) {
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

type readFailureApp struct {
	core.App
	err error
}

func (app readFailureApp) FindRecordById(any, string, ...func(*dbx.SelectQuery) error) (*core.Record, error) {
	return nil, app.err
}

func (app readFailureApp) FindRecordsByFilter(any, string, string, int, int, ...dbx.Params) ([]*core.Record, error) {
	return nil, app.err
}

func TestReadFailuresPropagateAcrossDomainMappings(t *testing.T) {
	store := testStore(t)
	wantErr := errors.New("database read failed")
	failingStore := New(readFailureApp{App: store.App, err: wantErr})
	device := domain.Device{ID: "device", BaselineID: "snapshot"}
	if _, err := failingStore.Device(device.ID); !errors.Is(err, wantErr) {
		t.Fatalf("device query hid its read error: %v", err)
	}
	if _, err := failingStore.Devices(); !errors.Is(err, wantErr) {
		t.Fatalf("inventory query hid its read error: %v", err)
	}
	if _, err := failingStore.Baseline(device); !errors.Is(err, wantErr) {
		t.Fatalf("baseline query hid its read error: %v", err)
	}
	if _, err := failingStore.ChannelNotes(device.ID); !errors.Is(err, wantErr) {
		t.Fatalf("channel note query hid its read error: %v", err)
	}
	if _, err := failingStore.Snapshots(device.ID); !errors.Is(err, wantErr) {
		t.Fatalf("snapshot query hid its read error: %v", err)
	}
	if _, err := failingStore.OrganizationOptions(); !errors.Is(err, wantErr) {
		t.Fatalf("organization query hid its read error: %v", err)
	}
	device.BaselineID = ""
	if baseline, err := failingStore.Baseline(device); err != nil || baseline != nil {
		t.Fatalf("absent baseline unexpectedly required a query: %v, %v", baseline, err)
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
