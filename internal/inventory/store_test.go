package inventory

import (
	"testing"

	"github.com/pocketbase/pocketbase"
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
	device, err := store.SaveReading(reading, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Metadata(device.Id, "Bedroom", "lighting", "Button beside door"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveChannelNote(device.Id, "switch:0", "Ceiling lights"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkBaseline(device.Id); err != nil {
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
	if err := store.MarkBaseline(device.Id); err == nil {
		t.Fatal("approved a baseline without an available configuration capture")
	}
}

func TestDiffDistinguishesMissingFromNullAndFalse(t *testing.T) {
	diff := Differences(map[string]any{"input": map[string]any{"state": nil, "enable": false}}, map[string]any{"input": map[string]any{"enable": true}})
	if len(diff) != 2 || diff[1].Before != "null" || diff[1].After != "Not present" {
		t.Fatalf("unexpected diff: %+v", diff)
	}
}
