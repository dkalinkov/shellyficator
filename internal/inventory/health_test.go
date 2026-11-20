package inventory

import (
	"reflect"
	"testing"
	"time"

	"shellyficator/internal/domain"
)

func TestOfflineRequiresBothRepeatedFailuresAndGracePeriod(t *testing.T) {
	store := testStore(t)
	for _, test := range []struct {
		name      string
		age       time.Duration
		failures  int
		reachable bool
	}{
		{"two failures after a long pause", 2 * time.Minute, 2, true},
		{"rapid manual refreshes", 10 * time.Second, 4, true},
		{"sustained outage", 2 * time.Minute, 3, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			reading := domain.Reading{
				Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4},
				Status:   map[string]any{"switch:0": map[string]any{"output": false}},
				Config:   map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}},
			}
			device, err := store.SaveReading(reading, true)
			if err != nil {
				t.Fatal(err)
			}
			record, err := store.App.FindRecordById("devices", device.ID)
			if err != nil {
				t.Fatal(err)
			}
			record.Set("last_seen", time.Now().UTC().Add(-test.age))
			record.Set("last_checked", time.Now().UTC().Add(-test.age))
			if err := store.App.Save(record); err != nil {
				t.Fatal(err)
			}
			before, _ := store.Device(device.ID)
			for i := 0; i < test.failures; i++ {
				if err := store.Failed(device.ID, "timeout", false); err != nil {
					t.Fatal(err)
				}
			}
			after, err := New(store.App).Device(device.ID)
			if err != nil {
				t.Fatal(err)
			}
			if after.Reachable != test.reachable || after.ConsecutiveFailures != test.failures || after.StatusCurrent() || after.ConfigCurrent() {
				t.Fatalf("incorrect persisted health: reachable=%v failures=%d", after.Reachable, after.ConsecutiveFailures)
			}
			if !reflect.DeepEqual(after.Status, before.Status) || !reflect.DeepEqual(after.Config, before.Config) || !after.LastSeen.Equal(before.LastSeen) || !after.LastStatusRead.Equal(before.LastStatusRead) || !after.LastCaptured.Equal(before.LastCaptured) {
				t.Fatal("failed check changed cached readings or their read times")
			}
			if after.LastChecked == before.LastChecked {
				t.Fatal("failed check did not record its time")
			}
			recovered, err := store.SaveReading(reading, false)
			if err != nil {
				t.Fatal(err)
			}
			if !recovered.StatusCurrent() || !recovered.ConfigCurrent() || recovered.ConsecutiveFailures != 0 || recovered.Problem != "" || time.Since(recovered.LastSeen) > time.Second {
				t.Fatal("successful contact did not immediately recover health")
			}
		})
	}
}

func TestIdentityMismatchIsImmediateAndSurvivesLaterTimeout(t *testing.T) {
	store := testStore(t)
	reading := domain.Reading{Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Status: map[string]any{}, Config: map[string]any{}}
	device, err := store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Failed(device.ID, "wrong device", true); err != nil {
		t.Fatal(err)
	}
	if err := store.Failed(device.ID, "timeout", false); err != nil {
		t.Fatal(err)
	}
	failed, _ := store.Device(device.ID)
	if failed.Reachable || !failed.IdentityMismatch || failed.MAC != "AABBCCDDEEFF" || failed.Problem != "Address now belongs to a different device. Rediscover this device." {
		t.Fatal("identity error delayed, hidden, or rebound the device")
	}
	reading.Config = nil
	recovered, err := store.SaveReading(reading, false)
	if err != nil || !recovered.StatusCurrent() || recovered.IdentityMismatch || recovered.ConfigCurrent() || recovered.Config == nil {
		t.Fatal("status recovery failed to clear the identity error or changed the saved configuration")
	}
}

func TestStatusFailurePreservesItsOwnReadTime(t *testing.T) {
	store := testStore(t)
	reading := domain.Reading{Identity: domain.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Status: map[string]any{"switch:0": map[string]any{"output": true}}}
	device, err := store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.App.FindRecordById("devices", device.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.Set("last_status_read", time.Now().UTC().Add(-time.Minute))
	if err := store.App.Save(record); err != nil {
		t.Fatal(err)
	}
	before, _ := store.Device(device.ID)
	reading.Status, reading.Problem = nil, "Status read failed"
	after, err := store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	if !after.Reachable || after.StatusCurrent() || after.LastStatusRead != before.LastStatusRead || !reflect.DeepEqual(after.Status, before.Status) || after.LastSeen == after.LastStatusRead {
		t.Fatal("failed status read changed cached status or claimed a newer status time")
	}
}
