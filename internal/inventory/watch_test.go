package inventory

import (
	"reflect"
	"shellyficator/internal/domain"
	"testing"
)

func TestWatchPreferencesPersistWithoutChangingDevices(t *testing.T) {
	store := testStore(t)
	device, err := store.ApproveReading(domain.Reading{Identity: domain.Identity{ID: "test", MAC: "AABBCCDDEEFF", Gen: 4}, Config: map[string]any{"input:0": map[string]any{"type": "button"}}})
	if err != nil {
		t.Fatal(err)
	}
	device, err = store.Device(device.ID)
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := store.WatchPreferences()
	if err != nil || !defaults.ForDevice(device.ID).Warn(domain.WatchButtons) || !defaults.ForDevice(device.ID).Warn(domain.WatchPower) || defaults.ForDevice(device.ID).Warn(domain.WatchOther) {
		t.Fatalf("incorrect defaults: %v %v", defaults, err)
	}
	if err := store.SaveWatchPolicy("", domain.WatchPolicy{}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveWatchPolicy(device.ID, domain.WatchPolicy{domain.WatchTimers: true}); err != nil {
		t.Fatal(err)
	}
	// Reopen storage so persisted policy, rather than an in-memory cache, is tested.
	if err := store.App.ResetBootstrapState(); err != nil {
		t.Fatal(err)
	}
	if err := store.App.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	restored, err := New(store.App).WatchPreferences()
	if err != nil || restored.ForDevice("").Warn(domain.WatchButtons) || !restored.ForDevice(device.ID).Warn(domain.WatchTimers) {
		t.Fatalf("preferences did not survive reopening: %v %v", restored, err)
	}
	if err := store.SaveWatchPolicy(device.ID, nil); err != nil {
		t.Fatal(err)
	}
	inherited, err := store.WatchPreferences()
	if err != nil || inherited.ForDevice(device.ID).Warn(domain.WatchTimers) {
		t.Fatal("override was not removed")
	}
	after, err := store.Device(device.ID)
	if err != nil || !reflect.DeepEqual(after, device) {
		t.Fatal("preference update mutated device")
	}
	snapshots, err := store.Snapshots(device.ID)
	if err != nil || len(snapshots) != 1 {
		t.Fatal("preference update mutated snapshots")
	}
	if err := store.SaveWatchPolicy("missing", domain.WatchPolicy{}); err == nil {
		t.Fatal("nonexistent device accepted")
	}
	if err := store.SaveWatchPolicy("", domain.WatchPolicy{"unknown": true}); err == nil {
		t.Fatal("unknown group accepted")
	}
}
