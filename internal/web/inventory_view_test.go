package web

import (
	"strings"
	"testing"

	"shellyficator/internal/domain"
)

func TestInventoryPresentation(t *testing.T) {
	viewFor := func(device domain.Device, baseline map[string]any) DeviceTable {
		return tableDevice(device, baseline, domain.CompareConfiguration(baseline, device.Config, nil), firmwareView(device))
	}
	record := func(status map[string]any) domain.Device {
		return domain.Device{
			Reachable: true, StatusAvailable: true, ConfigAvailable: true, Status: status,
			Config: map[string]any{
				"switch:0": map[string]any{"in_mode": "detached"},
				"input:0":  map[string]any{"type": "button", "enable": true},
				"switch:1": map[string]any{"in_mode": "follow"},
				"input:1":  map[string]any{"type": "switch", "enable": true},
			},
		}
	}
	status := map[string]any{
		"switch:0": map[string]any{"output": true},
		"switch:1": map[string]any{"output": false},
		"input:0":  map[string]any{"state": nil},
		"input:1":  map[string]any{"state": false},
		"wifi":     map[string]any{"rssi": -55.0},
	}

	t.Run("physical and relay state are independent and preserve false and null", func(t *testing.T) {
		view := viewFor(record(status), nil)
		if view.SettingsWarning != "" {
			t.Fatalf("available settings displayed as unavailable: %+v", view)
		}
		if view.ConnectivityIcon != "check-circle" || view.Inputs[0].State != "Not reported" || view.Inputs[0].Output != "On" || view.Inputs[1].State != "Off" || view.Inputs[1].Output != "Off" {
			t.Fatalf("input state or relay state lost: %+v", view.Inputs)
		}
	})
	t.Run("input identity follows explicit relay mapping", func(t *testing.T) {
		r := record(map[string]any{"switch:3": map[string]any{"output": true}, "input:9": map[string]any{"state": false}})
		r.Config = map[string]any{"switch:3": map[string]any{"input_id": float64(9), "in_mode": "follow"}, "input:9": map[string]any{"type": "switch"}}
		view := viewFor(r, nil)
		if len(view.Inputs) != 1 || view.Inputs[0].ID != "9" || view.Inputs[0].RelayKey != "switch:3" || view.Inputs[0].Mode != "Follow" || view.Inputs[0].State != "Off" || view.Inputs[0].Output != "On" {
			t.Fatalf("wrong physical input associated with relay: %+v", view.Inputs)
		}
	})
	t.Run("detached is only highlighted when it differs from baseline", func(t *testing.T) {
		r := record(status)
		without := viewFor(r, nil)
		if without.Inputs[0].Changed || without.Inputs[0].Mode != "Detached" {
			t.Fatalf("detached treated as an intrinsic fault: %+v", without.Inputs)
		}
		baseline := map[string]any{
			"switch:0": map[string]any{"in_mode": "momentary"},
			"input:0":  map[string]any{"type": "button", "enable": true},
			"switch:1": map[string]any{"in_mode": "follow"},
			"input:1":  map[string]any{"type": "switch", "enable": true},
		}
		view := viewFor(r, baseline)
		if !view.Inputs[0].Changed || view.Inputs[1].Changed || !strings.Contains(view.Inputs[0].Info, "momentary → detached") || view.ConfigurationTone != "warning" {
			t.Fatalf("changed channel was not identified: %+v", view)
		}
	})
	t.Run("offline readings remain cached and have no current states", func(t *testing.T) {
		r := record(status)
		r.Reachable = false
		view := viewFor(r, map[string]any{})
		if view.Connectivity != "Offline" || view.ConnectivityIcon != "close-circle" || view.SettingsWarning != "Settings unavailable · showing saved copy" || !view.Inputs[0].Changed || view.Configuration != "6 changes" || view.ConfigurationTone != "muted" || view.Inputs[0].State != "Not reported" || view.Inputs[0].Output != "On" || view.Inputs[0].OutputTone != "muted" || !strings.Contains(view.Inputs[0].StatusInfo, "Last reported") || view.SignalTone != "muted" || view.SignalValue != "-55" {
			t.Fatalf("stale values displayed as current: %+v", view)
		}
	})
	t.Run("missed checks retain muted readings and identity mismatches stay explicit", func(t *testing.T) {
		r := record(status)
		r.ConsecutiveFailures = 1
		r.StatusAvailable = false
		r.ConfigAvailable = false
		r.Status = map[string]any{"switch:0": map[string]any{"output": true}, "input:0": map[string]any{"state": false}}
		view := viewFor(r, nil)
		if view.Connectivity != "Last seen" || view.ConnectivityTone != "muted" || view.Inputs[0].Output != "On" || view.Inputs[0].State != "Off" || view.Inputs[0].OutputTone != "muted" {
			t.Fatalf("missed check lost cached values or displayed them as current: %+v", view)
		}
		r.IdentityMismatch = true
		r.Reachable = false
		if view := viewFor(r, nil); view.Connectivity != "Identity mismatch" || view.ConnectivityTone != "error" {
			t.Fatalf("identity mismatch hidden by ordinary outage: %+v", view)
		}
	})
	t.Run("status and config failures are presented independently", func(t *testing.T) {
		r := record(status)
		r.ConfigAvailable = false
		view := viewFor(r, map[string]any{})
		if view.Inputs[0].Output != "On" || view.Inputs[0].OutputTone != "ok" || view.SettingsWarning != "Settings unavailable · showing saved copy" || view.Configuration != "6 changes" || view.ConfigurationTone != "muted" || !strings.Contains(view.Inputs[0].Info, view.SettingsWarning) {
			t.Fatalf("configuration failure masked valid status: %+v", view)
		}
		r.StatusAvailable = false
		view = viewFor(r, map[string]any{})
		if view.Inputs[0].Output != "On" || view.Inputs[0].OutputTone != "muted" || view.Connectivity != "Read failed" || view.ConnectivityTone != "warning" || view.ConnectivityIcon != "warning" {
			t.Fatalf("status failure displayed as healthy: %+v", view)
		}
	})
	t.Run("device faults change the health icon and keep fault details", func(t *testing.T) {
		r := record(map[string]any{"switch:0": map[string]any{"errors": []any{"overtemp"}}})
		view := viewFor(r, nil)
		if view.Connectivity != "Device fault" || view.ConnectivityTone != "error" || view.ConnectivityIcon != "warning" || !strings.Contains(view.ConnectionInfo, "switch:0: overtemp") {
			t.Fatalf("fault indication or details lost: %+v", view)
		}
		r.Reachable = false
		if offline := viewFor(r, nil); offline.ConnectivityIcon != "close-circle" || strings.Contains(offline.ConnectionInfo, "overtemp") {
			t.Fatalf("cached fault presented as current: %+v", offline)
		}
	})
	t.Run("settings never read do not claim a saved copy", func(t *testing.T) {
		r := record(status)
		r.Config = nil
		r.ConfigAvailable = false
		view := viewFor(r, nil)
		if view.SettingsWarning != "Settings unavailable" || view.Inputs[0].Output != "On" {
			t.Fatalf("missing configuration misrepresented: %+v", view)
		}
	})
	t.Run("restart warnings require a current boolean status", func(t *testing.T) {
		for _, test := range []struct {
			name                 string
			sys                  any
			reachable, available bool
			want                 bool
		}{
			{"required", map[string]any{"restart_required": true}, true, true, true},
			{"not required", map[string]any{"restart_required": false}, true, true, false},
			{"missing system status", nil, true, true, false},
			{"malformed field", map[string]any{"restart_required": "true"}, true, true, false},
			{"offline cached warning", map[string]any{"restart_required": true}, false, true, false},
			{"failed status cached warning", map[string]any{"restart_required": true}, true, false, false},
		} {
			t.Run(test.name, func(t *testing.T) {
				r := record(map[string]any{"sys": test.sys})
				r.Reachable = test.reachable
				r.StatusAvailable = test.available
				if view := viewFor(r, nil); view.RestartRequired != test.want {
					t.Fatalf("restart warning does not reflect current status: %+v", view)
				}
			})
		}
	})
}
