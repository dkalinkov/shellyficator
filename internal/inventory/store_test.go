package inventory

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/shelly"
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
	reading := shelly.Reading{Identity: shelly.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Address: "http://192.168.1.2", Status: map[string]any{"switch:0": map[string]any{"output": false}}, Config: map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}}}
	device, err := store.ApproveReading(reading)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Metadata(device.Id, "Bedroom", "lighting", "Button beside door"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveChannelNote(device.Id, "switch:0", "Ceiling lights"); err != nil {
		t.Fatal(err)
	}
	reading.Address = "http://192.168.1.99"
	reading.Config = map[string]any{"switch:0": map[string]any{"in_mode": "detached"}}
	device, err = store.SaveReading(reading, true)
	if err != nil {
		t.Fatal(err)
	}
	if device.GetString("notes") != "Button beside door" || device.GetString("room") != "Bedroom" || store.ChannelNotes(device.Id)["switch:0"] != "Ceiling lights" {
		t.Fatal("rediscovery discarded local information")
	}
	devices, _ := store.Devices()
	if len(devices) != 1 {
		t.Fatal("IP change created another device")
	}
	diff := Differences(store.Baseline(device), Object(device, "config"))
	if len(diff) != 1 || diff[0].After != `"detached"` {
		t.Fatalf("baseline not preserved or drift missed: %+v", diff)
	}
	if err := store.Failed(device.Id, "unreachable", false); err != nil {
		t.Fatal(err)
	}
	device, _ = store.Device(device.Id)
	if store.Baseline(device) == nil || Object(device, "config") == nil || device.GetBool("config_available") {
		t.Fatal("failed scan discarded history or presented it as current")
	}
	reading.Config = nil
	device, err = store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	if !device.GetBool("reachable") || !device.GetBool("status_available") || device.GetBool("config_available") {
		t.Fatal("status recovery presented the old configuration as newly checked")
	}
	if _, err := store.ApproveReading(reading); err == nil {
		t.Fatal("approved a baseline without an available configuration capture")
	}
}

func TestDiffDistinguishesMissingFromNullAndFalse(t *testing.T) {
	diff := Differences(map[string]any{"input": map[string]any{"state": nil, "enable": false}}, map[string]any{"input": map[string]any{"enable": true}})
	if len(diff) != 2 || diff[1].Before != "null" || diff[1].After != "Not present" {
		t.Fatalf("unexpected diff: %+v", diff)
	}
}

func TestApprovalUsesItsOwnCaptureAndOtherReadsPreserveIt(t *testing.T) {
	store := testStore(t)
	reading := shelly.Reading{Identity: shelly.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Config: map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}}}
	device, err := store.SaveReading(reading, true)
	if err != nil {
		t.Fatal(err)
	}
	// Snapshot ordering must not decide which fresh read an approval selects.
	snapshots, err := store.Snapshots(device.Id)
	if err != nil {
		t.Fatal(err)
	}
	snapshots[0].Set("captured", time.Now().UTC().Add(time.Hour))
	if err := store.App.Save(snapshots[0]); err != nil {
		t.Fatal(err)
	}
	reading.Config = map[string]any{"switch:0": map[string]any{"in_mode": "detached"}}
	device, err = store.ApproveReading(reading)
	if err != nil {
		t.Fatal(err)
	}
	baselineID := device.GetString("baseline")
	if baselineID == snapshots[0].Id || len(Differences(store.Baseline(device), reading.Config)) != 0 {
		t.Fatal("approval selected another capture instead of its own fresh read")
	}
	reading.Config = map[string]any{"switch:0": map[string]any{"in_mode": "follow"}}
	for _, capture := range []bool{false, true} {
		device, err = store.SaveReading(reading, capture)
		if err != nil || device.GetString("baseline") != baselineID || store.Baseline(device)["switch:0"].(map[string]any)["in_mode"] != "detached" {
			t.Fatal("a later read or capture changed the approved snapshot")
		}
	}
	before, err := store.Device(device.Id)
	if err != nil {
		t.Fatal(err)
	}
	reading.Config = nil
	if _, err := store.ApproveReading(reading); err == nil {
		t.Fatal("approval accepted an unavailable configuration")
	}
	after, err := store.Device(device.Id)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err = store.Snapshots(device.Id)
	if err != nil || len(snapshots) != 3 || after.GetString("baseline") != baselineID || after.GetString("config") != before.GetString("config") {
		t.Fatal("failed approval changed the baseline, readings, or capture history")
	}
}

func TestApprovalRollsBackCaptureWhenBaselineSaveFails(t *testing.T) {
	store := testStore(t)
	reading := shelly.Reading{Identity: shelly.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Config: map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}}}
	before, err := store.ApproveReading(reading)
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("baseline save failed")
	store.App.OnRecordUpdate("devices").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetString("baseline") != before.GetString("baseline") {
			return wantErr
		}
		return e.Next()
	})
	reading.Config = map[string]any{"switch:0": map[string]any{"in_mode": "detached"}}
	if _, err := store.ApproveReading(reading); !errors.Is(err, wantErr) {
		t.Fatalf("approval did not return the baseline save error: %v", err)
	}
	after, err := store.Device(before.Id)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := store.Snapshots(before.Id)
	if err != nil || len(snapshots) != 1 || after.GetString("baseline") != before.GetString("baseline") || after.GetString("config") != before.GetString("config") {
		t.Fatal("failed baseline save partially committed its capture or reading")
	}
}

func TestConcurrentFirstChannelNoteSavesUpdateOneRecord(t *testing.T) {
	store := testStore(t)
	device, err := store.SaveReading(shelly.Reading{Identity: shelly.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Config: map[string]any{"input:0": map[string]any{"type": "button"}}}, false)
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
			errors <- store.SaveChannelNote(device.Id, "input:0", fmt.Sprintf("Note %d", i))
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
	if count, err := store.App.CountRecords("channel_notes"); err != nil || count != 1 || store.ChannelNotes(device.Id)["input:0"] == "" {
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
	reading := shelly.Reading{Identity: shelly.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Config: map[string]any{"input:0": map[string]any{"type": "button"}}}
	device, err := store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("record lookup failed")
	failingStore := New(lookupFailureApp{App: store.App, err: wantErr})
	if _, err := failingStore.SaveReading(reading, true); !errors.Is(err, wantErr) {
		t.Fatalf("reading save hid the lookup error: %v", err)
	}
	if err := failingStore.SaveChannelNote(device.Id, "input:0", "Unsaved"); !errors.Is(err, wantErr) {
		t.Fatalf("channel note save hid the lookup error: %v", err)
	}
	for collection, wantCount := range map[string]int64{"devices": 1, "snapshots": 0, "channel_notes": 0} {
		if count, err := store.App.CountRecords(collection); err != nil || count != wantCount {
			t.Fatalf("lookup failure changed %s records: count=%d err=%v", collection, count, err)
		}
	}
}
