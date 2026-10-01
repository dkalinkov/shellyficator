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
		if view.Inputs[0].State != "Not reported" || view.Inputs[0].Output != "On" || view.Inputs[1].State != "Off" || view.Inputs[1].Output != "Off" {
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
		if view.Connectivity != "Offline" || !view.CachedInputs || view.Inputs[0].Changed || view.Configuration != "Unavailable" || view.Inputs[0].State != "—" || view.Inputs[0].Output != "—" || view.SignalTone != "muted" || view.SignalValue != "-55" {
			t.Fatalf("stale values displayed as current: %+v", view)
		}
	})
	t.Run("status and config failures are presented independently", func(t *testing.T) {
		r := record(status)
		r.Set("config_available", false)
		view := tableDevice(r, map[string]any{}, 4)
		if view.Inputs[0].Output != "On" || !view.CachedInputs || view.Configuration != "Unavailable" {
			t.Fatalf("configuration failure masked valid status: %+v", view)
		}
		r.Set("status_available", false)
		view = tableDevice(r, map[string]any{}, 4)
		if view.Inputs[0].Output != "—" || view.Connectivity != "Read failed" || view.ConnectivityTone != "warning" {
			t.Fatalf("status failure displayed as healthy: %+v", view)
		}
	})
}
