package domain

import "testing"

func TestConfigurationComparison(t *testing.T) {
	before := map[string]any{
		"sys":      map[string]any{"cfg_rev": 1, "device": map[string]any{"fw_id": "old", "mac": "old", "name": "Kitchen", "profile": "switch"}},
		"switch:0": map[string]any{"id": 0, "in_mode": "follow", "initial_state": "off", "auto_off": false},
		"wifi":     map[string]any{"sta": map[string]any{"ssid": "home"}},
	}
	after := map[string]any{
		"sys":      map[string]any{"cfg_rev": 2, "device": map[string]any{"fw_id": "new", "mac": "new", "name": "Hall", "profile": "cover"}},
		"switch:0": map[string]any{"id": 1, "in_mode": "detached", "initial_state": "restore_last", "auto_off": true},
		"wifi":     map[string]any{"sta": map[string]any{"ssid": "other"}},
	}
	comparison := CompareConfiguration(before, after)
	if len(comparison.Settings) != 3 || len(comparison.Informational) != 7 {
		t.Fatalf("incorrect classification: %+v", comparison)
	}
	if len(Differences(before, after)) != 10 {
		t.Fatal("raw comparison lost metadata")
	}
}

func TestConfigurationMetadataRulesAreScoped(t *testing.T) {
	for _, path := range []string{
		"sys / device / enhanced_security", "sys / device / tls_check_cert_validity_time",
		"wifi / ap / is_open", "wifi / sta / ipv4mode", "mqtt / enable", "cloud / enable",
		"sys / location / tz", "sys / ui_data / label", "switch:0 / power_limit",
		"custom / cfg_rev", "custom / fw_id", "custom / mac", "custom / id",
		"switch:0 / nested / id", "future:0 / new_setting",
	} {
		t.Run(path, func(t *testing.T) {
			if configurationMetadata(path) {
				t.Fatal("user setting or unknown field was silenced")
			}
		})
	}
}

func TestConfigurationComparisonRetainsAddedAndRemovedSettings(t *testing.T) {
	before := map[string]any{"input:0": map[string]any{"enable": false}, "sys": map[string]any{"cfg_rev": 1}}
	after := map[string]any{"input:1": map[string]any{"enable": nil}, "sys": map[string]any{"cfg_rev": 2}}
	comparison := CompareConfiguration(before, after)
	if len(comparison.Settings) != 2 || len(comparison.Informational) != 1 {
		t.Fatalf("incorrect classification: %+v", comparison)
	}
	removed, added := comparison.Settings[0], comparison.Settings[1]
	if !removed.Before.Present || removed.After.Present || added.Before.Present || !added.After.Present || added.After.Value != nil {
		t.Fatalf("presence lost: %+v", comparison.Settings)
	}
}

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
		{"script:1 / enable", WatchAutomations}, {"sys / cfg_rev", WatchMetadata}, {"new:0 / value", WatchOther},
	} {
		if got := ConfigurationGroup(test.path); got != test.group {
			t.Errorf("%s: got %s, want %s", test.path, got, test.group)
		}
	}
	before := map[string]any{"switch:0": map[string]any{"in_mode": "detached", "initial_state": "off", "auto_off": false}, "new": false}
	after := map[string]any{"switch:0": map[string]any{"in_mode": "follow", "initial_state": "on", "auto_off": true}, "new": true}
	for _, test := range []struct {
		policy   WatchPolicy
		warnings int
	}{
		{nil, 2}, {WatchPolicy{}, 0}, {WatchPolicy{WatchTimers: true}, 1}, {WatchPolicy{WatchOther: true}, 1},
	} {
		got := CompareConfiguration(before, after, test.policy)
		if len(got.Settings) != test.warnings || len(got.Settings)+len(got.Informational) != 4 {
			t.Fatalf("classification lost changes: %+v", got)
		}
	}
}
