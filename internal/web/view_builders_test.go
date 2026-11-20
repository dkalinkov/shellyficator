package web

import (
	"bytes"
	"strings"
	"testing"

	"shellyficator/internal/domain"
)

func TestDevicePagePreservesDifferencePresenceAndValues(t *testing.T) {
	page := devicePage(PageMeta{}, domain.DeviceDetails{InventoryEntry: domain.InventoryEntry{
		Device: domain.Device{
			ID: "device", Reachable: true, ConfigAvailable: true,
			Config: map[string]any{"added": false, "nullable": nil, "zero": float64(0)},
		},
		Baseline: map[string]any{"removed": false, "nullable": false, "zero": float64(1)},
	}}, domain.WatchPolicy{domain.WatchOther: true})
	templates, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	if err := templates.ExecuteTemplate(&body, "detail-configuration", page); err != nil {
		t.Fatal(err)
	}
	content := renderedText(body.String())
	for _, row := range []string{"added Not present false", "removed false Not present", "nullable false null", "zero 1 0"} {
		if !strings.Contains(content, row) {
			t.Errorf("comparison did not display %q", row)
		}
	}
}

func TestRecordedChangesAreShownOnlyForCurrentConfiguration(t *testing.T) {
	entry := domain.InventoryEntry{
		Device:   domain.Device{ID: "device", Reachable: true, Config: map[string]any{"sys": map[string]any{"cfg_rev": 2}}},
		Baseline: map[string]any{"sys": map[string]any{"cfg_rev": 1}},
	}
	templates, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}
	for _, fresh := range []bool{true, false} {
		entry.Device.ConfigAvailable = fresh
		page := devicePage(PageMeta{}, domain.DeviceDetails{InventoryEntry: entry}, nil)
		var body bytes.Buffer
		if err := templates.ExecuteTemplate(&body, "detail-configuration", page); err != nil {
			t.Fatal(err)
		}
		if shown := strings.Contains(renderedText(body.String()), "Recorded changes (1)"); shown != fresh {
			t.Fatalf("recorded changes shown=%t for current settings=%t", shown, fresh)
		}
	}
}

func TestWatchPreferencesApplyAcrossInventoryAndDetail(t *testing.T) {
	entry := domain.InventoryEntry{
		Device: domain.Device{ID: "one", Reachable: true, ConfigAvailable: true, Config: map[string]any{
			"input:0": map[string]any{"type": "switch"}, "switch:0": map[string]any{"in_mode": "follow", "auto_off": true},
		}},
		Baseline: map[string]any{"input:0": map[string]any{"type": "button"}, "switch:0": map[string]any{"in_mode": "detached", "auto_off": false}},
	}
	for _, test := range []struct {
		name        string
		preferences domain.WatchPreferences
		warnings    int
		label       string
	}{
		{"defaults", nil, 2, "2 changes"},
		{"record all", domain.WatchPreferences{"": domain.WatchPolicy{}}, 0, "Recorded changes"},
		{"timers only", domain.WatchPreferences{"": domain.WatchPolicy{domain.WatchTimers: true}}, 1, "1 change"},
		{"device override", domain.WatchPreferences{"": domain.WatchPolicy{}, "one": domain.WatchPolicy{domain.WatchButtons: true}}, 2, "2 changes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			inventory := inventoryPage(PageMeta{}, []domain.InventoryEntry{entry}, inventoryFilter{}, "", test.preferences)
			detail := devicePage(PageMeta{}, domain.DeviceDetails{InventoryEntry: entry}, test.preferences.ForDevice("one"))
			if len(detail.Differences) != test.warnings || len(detail.InformationalDifferences) != 3-test.warnings {
				t.Fatalf("wrong differences: %+v", detail)
			}
			wanted := test.warnings > 0
			if (inventory.Changed > 0) != wanted || inventory.Devices[0].Table.InputsChanged != wanted || detail.Detail.Table.InputsChanged != wanted || detail.Detail.ShowDifferences != wanted {
				t.Fatal("warnings disagree across app")
			}
			if inventory.Devices[0].Table.Configuration != test.label || detail.Detail.Table.Configuration != test.label {
				t.Fatalf("inventory and detail should both report %q", test.label)
			}
		})
	}
}
