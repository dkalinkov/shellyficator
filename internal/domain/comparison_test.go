package domain

import (
	"slices"
	"testing"
)

func TestWatchCategoriesAndPolicies(t *testing.T) {
	for _, test := range []struct {
		path  string
		group WatchGroup
	}{
		{"input:0 / type", WatchButtons}, {"input:0 / invert", WatchButtons}, {"input:0 / enable", WatchButtons},
		{"input:0 / name", WatchOther}, {"switch:0 / input_id", WatchButtons}, {"switch:0 / in_mode", WatchButtons},
		{"cover:0 / swap_inputs", WatchButtons}, {"sys / device / profile", WatchButtons},
		{"switch:0 / initial_state", WatchPower}, {"switch:0 / power_restore_delay", WatchPower},
		{"switch:0 / auto_off_delay", WatchTimers}, {"switch:0 / power_limit", WatchOther},
		{"wifi / sta / ssid", WatchConnectivity}, {"sys / device / enhanced_security", WatchConnectivity},
		{"sys / device / tls_check_cert_validity_time", WatchConnectivity}, {"mqtt / enable", WatchConnectivity},
		{"cloud / enable", WatchConnectivity}, {"sys / location / tz", WatchOther},
		{"script:1 / enable", WatchAutomations}, {"new:0 / value", WatchOther},
		{"sys / cfg_rev", WatchMetadata}, {"sys / device / fw_id", WatchMetadata},
		{"sys / device / mac", WatchMetadata}, {"switch:0 / id", WatchMetadata},
		{"custom / cfg_rev", WatchOther}, {"custom / fw_id", WatchOther},
		{"custom / mac", WatchOther}, {"custom / id", WatchOther}, {"switch:0 / nested / id", WatchOther},
	} {
		if got := ConfigurationGroup(test.path); got != test.group {
			t.Errorf("%s: got %s, want %s", test.path, got, test.group)
		}
	}
	before := map[string]any{"input:0": map[string]any{"enable": false}, "switch:0": map[string]any{"initial_state": "off", "auto_off": false}, "new": false}
	after := map[string]any{"input:1": map[string]any{"enable": nil}, "switch:0": map[string]any{"initial_state": "on", "auto_off": true}, "new": true}
	for _, test := range []struct {
		policy WatchPolicy
		paths  []string
	}{
		{nil, []string{"input:0 / enable", "input:1 / enable", "switch:0 / initial_state"}},
		{WatchPolicy{}, nil},
		{WatchPolicy{WatchTimers: true}, []string{"switch:0 / auto_off"}},
		{WatchPolicy{WatchOther: true}, []string{"new"}},
	} {
		got := CompareConfiguration(before, after, test.policy)
		var paths []string
		for _, warning := range got.Settings {
			paths = append(paths, warning.Path)
		}
		if !slices.Equal(paths, test.paths) || len(paths)+len(got.Informational) != 5 {
			t.Fatalf("wrong changes for policy %v: %+v", test.policy, got)
		}
	}
}
