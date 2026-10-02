package web

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	_ "shellyficator/migrations"
)

func TestInventoryPresentation(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	collection, err := app.FindCollectionByNameOrId("devices")
	if err != nil {
		t.Fatal(err)
	}
	record := func(status map[string]any) *core.Record {
		r := core.NewRecord(collection)
		r.Set("reachable", true)
		r.Set("status_available", true)
		r.Set("config_available", true)
		r.Set("status", status)
		r.Set("config", map[string]any{
			"switch:0": map[string]any{"in_mode": "detached"},
			"input:0":  map[string]any{"type": "button", "enable": true},
			"switch:1": map[string]any{"in_mode": "follow"},
			"input:1":  map[string]any{"type": "switch", "enable": true},
		})
		return r
	}
	status := map[string]any{
		"switch:0": map[string]any{"output": true},
		"switch:1": map[string]any{"output": false},
		"input:0":  map[string]any{"state": nil},
		"input:1":  map[string]any{"state": false},
		"wifi":     map[string]any{"rssi": -55.0},
	}

	t.Run("physical and relay state are independent and preserve false and null", func(t *testing.T) {
		view := tableDevice(record(status), nil, 0)
		if view.SettingsWarning != "" {
			t.Fatalf("available settings displayed as unavailable: %+v", view)
		}
		if view.ConnectivityIcon != "check-circle" || view.Inputs[0].State != "Not reported" || view.Inputs[0].Output != "On" || view.Inputs[1].State != "Off" || view.Inputs[1].Output != "Off" {
			t.Fatalf("input state or relay state lost: %+v", view.Inputs)
		}
	})
	t.Run("input identity follows explicit relay mapping", func(t *testing.T) {
		r := record(map[string]any{"switch:3": map[string]any{"output": true}, "input:9": map[string]any{"state": false}})
		r.Set("config", map[string]any{"switch:3": map[string]any{"input_id": 9, "in_mode": "follow"}, "input:9": map[string]any{"type": "switch"}})
		view := tableDevice(r, nil, 0)
		if len(view.Inputs) != 1 || view.Inputs[0].ID != "9" || view.Inputs[0].RelayKey != "switch:3" || view.Inputs[0].Mode != "Follow" || view.Inputs[0].State != "Off" || view.Inputs[0].Output != "On" {
			t.Fatalf("wrong physical input associated with relay: %+v", view.Inputs)
		}
	})
	t.Run("detached is only highlighted when it differs from baseline", func(t *testing.T) {
		r := record(status)
		without := tableDevice(r, nil, 0)
		if without.Inputs[0].Changed || without.Inputs[0].Mode != "Detached" {
			t.Fatalf("detached treated as an intrinsic fault: %+v", without.Inputs)
		}
		baseline := map[string]any{
			"switch:0": map[string]any{"in_mode": "momentary"},
			"input:0":  map[string]any{"type": "button", "enable": true},
			"switch:1": map[string]any{"in_mode": "follow"},
			"input:1":  map[string]any{"type": "switch", "enable": true},
		}
		view := tableDevice(r, baseline, 1)
		if !view.Inputs[0].Changed || view.Inputs[1].Changed || !strings.Contains(view.Inputs[0].Info, "momentary → detached") || view.ConfigurationTone != "warning" {
			t.Fatalf("changed channel was not identified: %+v", view)
		}
	})
	t.Run("offline readings remain cached and have no current states", func(t *testing.T) {
		r := record(status)
		r.Set("reachable", false)
		view := tableDevice(r, map[string]any{}, 4)
		if view.Connectivity != "Offline" || view.ConnectivityIcon != "close-circle" || view.SettingsWarning != "Settings unavailable · showing saved copy" || view.Inputs[0].Changed || view.Configuration != "Unavailable" || view.Inputs[0].State != "—" || view.Inputs[0].Output != "—" || view.SignalTone != "muted" || view.SignalValue != "-55" {
			t.Fatalf("stale values displayed as current: %+v", view)
		}
	})
	t.Run("status and config failures are presented independently", func(t *testing.T) {
		r := record(status)
		r.Set("config_available", false)
		view := tableDevice(r, map[string]any{}, 4)
		if view.Inputs[0].Output != "On" || view.SettingsWarning != "Settings unavailable · showing saved copy" || view.Configuration != "Unavailable" || !strings.Contains(view.Inputs[0].Info, view.SettingsWarning) {
			t.Fatalf("configuration failure masked valid status: %+v", view)
		}
		r.Set("status_available", false)
		view = tableDevice(r, map[string]any{}, 4)
		if view.Inputs[0].Output != "—" || view.Connectivity != "Read failed" || view.ConnectivityTone != "warning" || view.ConnectivityIcon != "warning" {
			t.Fatalf("status failure displayed as healthy: %+v", view)
		}
	})
	t.Run("device faults change the health icon and keep fault details", func(t *testing.T) {
		r := record(map[string]any{"switch:0": map[string]any{"errors": []string{"overtemp"}}})
		view := tableDevice(r, nil, 0)
		if view.Connectivity != "Device fault" || view.ConnectivityTone != "error" || view.ConnectivityIcon != "warning" || !strings.Contains(view.ConnectionInfo, "switch:0: overtemp") {
			t.Fatalf("fault indication or details lost: %+v", view)
		}
		r.Set("reachable", false)
		if offline := tableDevice(r, nil, 0); offline.ConnectivityIcon != "close-circle" || strings.Contains(offline.ConnectionInfo, "overtemp") {
			t.Fatalf("cached fault presented as current: %+v", offline)
		}
	})
	t.Run("settings never read do not claim a saved copy", func(t *testing.T) {
		r := record(status)
		r.Set("config", nil)
		r.Set("config_available", false)
		view := tableDevice(r, nil, 0)
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
			{"missing field", map[string]any{}, true, true, false},
			{"null field", map[string]any{"restart_required": nil}, true, true, false},
			{"malformed field", map[string]any{"restart_required": "true"}, true, true, false},
			{"offline cached warning", map[string]any{"restart_required": true}, false, true, false},
			{"failed status cached warning", map[string]any{"restart_required": true}, true, false, false},
		} {
			t.Run(test.name, func(t *testing.T) {
				r := record(map[string]any{"sys": test.sys})
				r.Set("reachable", test.reachable)
				r.Set("status_available", test.available)
				if view := tableDevice(r, nil, 0); view.RestartRequired != test.want {
					t.Fatalf("restart warning does not reflect current status: %+v", view)
				}
			})
		}
	})
}
