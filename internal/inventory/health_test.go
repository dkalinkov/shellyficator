package inventory

import (
	"testing"
	"time"

	"shellyficator/internal/shelly"
)

func TestOfflineRequiresBothRepeatedFailuresAndGracePeriod(t *testing.T) {
	store := testStore(t)
	for _, test := range []struct {
		name      string
		age       time.Duration
		failures  int
		reachable bool
	}{
		{"one failure after a long pause", 2 * time.Minute, 1, true},
		{"two failures after a long pause", 2 * time.Minute, 2, true},
		{"rapid manual refreshes", 10 * time.Second, 4, true},
		{"sustained outage", 2 * time.Minute, 3, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			reading := shelly.Reading{
				Identity: shelly.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4},
				Status:   map[string]any{"switch:0": map[string]any{"output": false}},
				Config:   map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}},
			}
			device, err := store.SaveReading(reading, true)
			if err != nil {
				t.Fatal(err)
			}
			device.Set("last_seen", time.Now().UTC().Add(-test.age))
			device.Set("last_checked", time.Now().UTC().Add(-test.age))
			if err := store.App.Save(device); err != nil {
				t.Fatal(err)
			}
			before, _ := store.Device(device.Id)
			for i := 0; i < test.failures; i++ {
				if err := store.Failed(device.Id, "timeout", false); err != nil {
					t.Fatal(err)
				}
			}
			// Load the saved health through a new store, rather than the old record.
			after, err := New(store.App).Device(device.Id)
			if err != nil {
				t.Fatal(err)
			}
			if after.GetBool("reachable") != test.reachable || after.GetInt("consecutive_failures") != test.failures || StatusCurrent(after) || ConfigCurrent(after) {
				t.Fatalf("incorrect persisted health: reachable=%v failures=%d", after.GetBool("reachable"), after.GetInt("consecutive_failures"))
			}
			for _, field := range []string{"status", "config", "last_seen", "last_status_read", "last_captured"} {
				if after.GetString(field) != before.GetString(field) {
					t.Fatalf("failed check changed saved %s", field)
				}
			}
			if after.GetString("last_checked") == before.GetString("last_checked") {
				t.Fatal("failed check did not record its time")
			}
			recovered, err := store.SaveReading(reading, false)
			if err != nil {
				t.Fatal(err)
			}
			if !StatusCurrent(recovered) || !ConfigCurrent(recovered) || recovered.GetInt("consecutive_failures") != 0 || recovered.GetString("problem") != "" || time.Since(recovered.GetDateTime("last_seen").Time()) > time.Second {
				t.Fatal("successful contact did not immediately recover health")
			}
		})
	}
}

func TestIdentityMismatchIsImmediateAndSurvivesLaterTimeout(t *testing.T) {
	store := testStore(t)
	reading := shelly.Reading{Identity: shelly.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Status: map[string]any{}, Config: map[string]any{}}
	device, err := store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Failed(device.Id, "wrong device", true); err != nil {
		t.Fatal(err)
	}
	if err := store.Failed(device.Id, "timeout", false); err != nil {
		t.Fatal(err)
	}
	failed, _ := store.Device(device.Id)
	if failed.GetBool("reachable") || !failed.GetBool("identity_mismatch") || failed.GetString("mac") != "AABBCCDDEEFF" || failed.GetString("problem") != "Address now belongs to a different device. Rediscover this device." {
		t.Fatal("identity error delayed, hidden, or rebound the device")
	}
	recovered, err := store.SaveReading(reading, false)
	if err != nil || !StatusCurrent(recovered) || recovered.GetBool("identity_mismatch") {
		t.Fatal("validated contact did not clear the identity error")
	}
}

func TestStatusFailurePreservesItsOwnReadTime(t *testing.T) {
	store := testStore(t)
	reading := shelly.Reading{Identity: shelly.Identity{ID: "test-device", MAC: "AABBCCDDEEFF", Gen: 4}, Status: map[string]any{"switch:0": map[string]any{"output": true}}}
	device, err := store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	device.Set("last_status_read", time.Now().UTC().Add(-time.Minute))
	if err := store.App.Save(device); err != nil {
		t.Fatal(err)
	}
	before, _ := store.Device(device.Id)
	reading.Status, reading.Problem = nil, "Status read failed"
	after, err := store.SaveReading(reading, false)
	if err != nil {
		t.Fatal(err)
	}
	if !after.GetBool("reachable") || StatusCurrent(after) || after.GetString("last_status_read") != before.GetString("last_status_read") || after.GetString("status") != before.GetString("status") || after.GetString("last_seen") == after.GetString("last_status_read") {
		t.Fatal("failed status read changed cached status or claimed a newer status time")
	}
}
