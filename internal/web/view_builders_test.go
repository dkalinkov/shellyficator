package web

import (
	"bytes"
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
	values := map[string]DifferenceView{}
	for _, difference := range page.Differences {
		values[difference.Path] = difference
	}
	for _, test := range []struct{ path, before, after string }{
		{"added", "Not present", "false"},
		{"removed", "false", "Not present"},
		{"nullable", "false", "null"},
		{"zero", "1", "0"},
	} {
		value := values[test.path]
		if value.Before != test.before || value.After != test.after {
			t.Fatalf("%s lost presence or value: %+v", test.path, value)
		}
	}
	templates, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	if err := templates.ExecuteTemplate(&body, "detail-configuration", page); err != nil {
		t.Fatal(err)
	}
	if text := renderedText(body.String()); text == "" {
		t.Fatal("difference table was not rendered")
	}
}

func TestConfigurationWarningsExcludeMetadata(t *testing.T) {
	baseline := map[string]any{
		"sys":      map[string]any{"cfg_rev": 1, "device": map[string]any{"fw_id": "old"}},
		"switch:0": map[string]any{"in_mode": "follow"},
	}
	for _, changedSetting := range []bool{false, true} {
		for _, fresh := range []bool{false, true} {
			mode := "follow"
			if changedSetting {
				mode = "detached"
			}
			entry := domain.InventoryEntry{
				Device: domain.Device{ID: "device", Reachable: true, ConfigAvailable: fresh,
					Config: map[string]any{
						"sys":      map[string]any{"cfg_rev": 2, "device": map[string]any{"fw_id": "new"}},
						"switch:0": map[string]any{"in_mode": mode},
					}},
				Baseline: baseline,
			}
			page := devicePage(PageMeta{}, domain.DeviceDetails{InventoryEntry: entry})
			inventory := inventoryPage(PageMeta{}, []domain.InventoryEntry{entry}, "", "", "", "")
			wantChanges := 0
			if changedSetting {
				wantChanges = 1
			}
			if len(page.Differences) != wantChanges || len(page.InformationalDifferences) != 2 || inventory.Changed != wantChanges {
				t.Fatalf("counts include metadata: detail=%+v inventory=%+v", page, inventory)
			}
			if page.Detail.ShowDifferences != (fresh && changedSetting) {
				t.Fatal("stale settings presented as current")
			}
			if !changedSetting && fresh && inventory.Devices[0].Table.Configuration != "Recorded changes" {
				t.Fatal("metadata-only change not acknowledged")
			}
			templates, err := parseTemplates()
			if err != nil {
				t.Fatal(err)
			}
			var body bytes.Buffer
			if err := templates.ExecuteTemplate(&body, "detail-configuration", page); err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(body.Bytes(), []byte("<details")) != fresh {
				t.Fatal("metadata disclosure missing or stale metadata shown as current")
			}
			// Approving current values clears both categories.
			entry.Baseline = entry.Device.Config
			approved := devicePage(PageMeta{}, domain.DeviceDetails{InventoryEntry: entry})
			if len(approved.Differences)+len(approved.InformationalDifferences) != 0 {
				t.Fatal("approval did not clear comparison")
			}
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
	}{
		{"defaults", nil, 2},
		{"record all", domain.WatchPreferences{"": domain.WatchPolicy{}}, 0},
		{"timers only", domain.WatchPreferences{"": domain.WatchPolicy{domain.WatchTimers: true}}, 1},
		{"device override", domain.WatchPreferences{"": domain.WatchPolicy{}, "one": domain.WatchPolicy{domain.WatchButtons: true}}, 2},
		{"other device override", domain.WatchPreferences{"": domain.WatchPolicy{}, "two": domain.WatchPolicy{domain.WatchButtons: true}}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			inventory := inventoryPage(PageMeta{}, []domain.InventoryEntry{entry}, "", "", "", "", test.preferences)
			detail := devicePage(PageMeta{}, domain.DeviceDetails{InventoryEntry: entry}, test.preferences.ForDevice("one"))
			if len(detail.Differences) != test.warnings || len(detail.InformationalDifferences) != 3-test.warnings {
				t.Fatalf("wrong differences: %+v", detail)
			}
			wanted := test.warnings > 0
			if (inventory.Changed > 0) != wanted || inventory.Devices[0].Table.InputsChanged != wanted || detail.Detail.Table.InputsChanged != wanted || detail.Detail.ShowDifferences != wanted {
				t.Fatal("warnings disagree across app")
			}
			if inventory.Devices[0].Table.Configuration != detail.Detail.Table.Configuration {
				t.Fatal("inventory/detail labels disagree")
			}
		})
	}
}
