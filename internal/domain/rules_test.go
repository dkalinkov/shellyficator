package domain

import (
	"reflect"
	"testing"
	"time"
)

func TestDifferencesRetainPresenceAndJSONValues(t *testing.T) {
	before := map[string]any{"input": map[string]any{"null": nil, "enabled": false, "zero": float64(0), "removed": "old"}}
	after := map[string]any{"input": map[string]any{"null": nil, "enabled": true, "zero": 0, "added": nil}}
	diff := Differences(before, after)
	if len(diff) != 3 {
		t.Fatalf("unexpected differences: %+v", diff)
	}
	if diff[0].Path != "input / added" || diff[0].Before.Present || !diff[0].After.Present || diff[0].After.Value != nil {
		t.Fatalf("added null lost its presence: %+v", diff[0])
	}
	if !diff[1].Before.Present || diff[1].Before.Value != false || diff[1].After.Value != true {
		t.Fatalf("boolean change lost its value: %+v", diff[1])
	}
	if !diff[2].Before.Present || diff[2].After.Present {
		t.Fatalf("removed setting lost its presence: %+v", diff[2])
	}
}

func TestFailedCheckAndRecoveryPreserveLocalAndCachedData(t *testing.T) {
	now := time.Now().UTC()
	d := Device{ID: "local-id", MAC: "AABBCCDDEEFF", Room: "Kitchen", Tags: []string{"Lighting"}, Notes: "Local note", BaselineID: "approved", Reachable: true, StatusAvailable: true, ConfigAvailable: true, LastSeen: now.Add(-2 * time.Minute), LastCaptured: now.Add(-3 * time.Minute), LastStatusRead: now.Add(-2 * time.Minute), Status: map[string]any{"output": false}, Config: map[string]any{"enabled": false}}
	for i := 0; i < 2; i++ {
		d = d.FailedCheck("timeout", false, now)
	}
	if !d.Reachable || d.StatusCurrent() || d.ConfigCurrent() {
		t.Fatal("ordinary misses skipped the offline grace policy")
	}
	d = d.FailedCheck("timeout", false, now)
	if d.Reachable {
		t.Fatal("sustained outage stayed reachable")
	}
	stale := d
	d = d.ApplyReading(Reading{Identity: Identity{ID: "reported-id", MAC: d.MAC, Gen: 4}, Status: map[string]any{"output": true}}, false, now)
	if !d.StatusCurrent() || d.ConfigCurrent() || d.ConsecutiveFailures != 0 || d.Problem != "" {
		t.Fatal("status recovery mixed status and configuration availability")
	}
	if d.ID != stale.ID || d.Room != stale.Room || d.Notes != stale.Notes || d.BaselineID != stale.BaselineID || !reflect.DeepEqual(d.Tags, stale.Tags) || !reflect.DeepEqual(d.Config, stale.Config) || !d.LastCaptured.Equal(stale.LastCaptured) {
		t.Fatal("reading replaced local metadata or unavailable configuration")
	}
	d = d.FailedCheck("identity", true, now)
	d = d.FailedCheck("timeout", false, now)
	if d.Reachable || !d.IdentityMismatch || d.Problem != "Address now belongs to a different device. Rediscover this device." {
		t.Fatal("subsequent timeout hid the identity mismatch")
	}
}

func TestCanonicalOrganizationPreservesExistingNames(t *testing.T) {
	options := Options([]Device{{Room: "Living Room", Tags: []string{"Wall control", "Lighting"}}})
	room, tags, err := CanonicalOrganization(" living   room ", "wall control, LIGHTING, Wall Control", options)
	if err != nil || room != "Living Room" || !reflect.DeepEqual(tags, []string{"Wall control", "Lighting"}) {
		t.Fatalf("unexpected organization: %q %v %v", room, tags, err)
	}
}
