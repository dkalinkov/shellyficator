package domain

import "testing"

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

func TestNetworkPrefix(t *testing.T) {
	for value, want := range map[string]string{
		"192.168.1.77/24": "192.168.1.0/24",
		" 10.0.0.0/24 ":   "10.0.0.0/24",
		"172.16.4.130/25": "172.16.4.128/25",
		"192.168.1.5/32":  "192.168.1.5/32",
		"192.168.0.0/16":  "",
		"8.8.8.0/24":      "",
		"fd00::/120":      "",
		"192.168.1.5":     "",
		"not a network":   "",
	} {
		if got := NetworkPrefix(value); got != want {
			t.Errorf("NetworkPrefix(%q) = %q, want %q", value, got, want)
		}
	}
}
