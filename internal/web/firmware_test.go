package web

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"shellyficator/internal/domain"
)

func TestFirmwareAvailabilityStates(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct {
		name, label, tone               string
		updates                         any
		installed                       string
		stale, available, known, staged bool
	}{
		{"stable", "Update available", "warning", map[string]any{"stable": map[string]any{"version": "2.2.0"}}, "2.1.0", false, true, true, false},
		{"beta", "Beta available", "blue", map[string]any{"beta": map[string]any{"version": "2.3.0-beta1"}}, "2.1.0", false, true, true, false},
		{"none", "No update reported", "muted", map[string]any{}, "2.1.0", false, false, true, false},
		{"unknown", "Not checked", "muted", nil, "2.1.0", false, false, false, false},
		{"invalid", "Not checked", "muted", "invalid", "2.1.0", false, false, false, false},
		{"installed offer", "No update reported", "muted", map[string]any{"stable": map[string]any{"version": "2.1.0"}}, "2.1.0", false, false, true, false},
		{"staged same version", "Update available", "warning", map[string]any{"stable": map[string]any{"version": "2.1.0", "nsteps": float64(0)}}, "2.1.0", false, true, true, true},
		{"stale", "Last reported · Update available", "muted", map[string]any{"stable": map[string]any{"version": "2.2.0"}}, "2.1.0", true, true, true, false},
		{"alternative firmware", "No update reported", "muted", map[string]any{"alt": map[string]any{"Zigbee": map[string]any{"stable": map[string]any{"version": "2.2.0"}}}}, "2.1.0", false, false, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			device := domain.Device{Firmware: test.installed, Reachable: !test.stale, StatusAvailable: !test.stale, LastStatusRead: now,
				Status: map[string]any{"sys": map[string]any{"available_updates": test.updates}}}
			view := firmwareView(device)
			if view.Label != test.label || view.Tone != test.tone || view.Available != test.available || view.Known != test.known || view.Staged != test.staged {
				t.Fatalf("incorrect firmware status: %+v", view)
			}
		})
	}
}

func TestFirmwareCheckPersistsErrorsAndEmptyResultsWithoutChangingBaseline(t *testing.T) {
	var response = `{"stable":{"version":"2.2.0"}}`
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4,"ver":"2.1.0"}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{"sys":{"unixtime":null,"available_updates":{"stable":{"version":"2.2.0"}}}}`))
		case "/rpc/Shelly.CheckForUpdate":
			_, _ = w.Write([]byte(response))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	})
	before, _ := s.store.Device(id)
	for _, body := range []string{`{"stable":{"version":"2.2.0"}}`, `{"code":-114,"message":"Download failed"}`, `{}`} {
		response = body
		event, recorder := detailEvent(s, id, http.MethodPost, "firmware/check", nil)
		if err := s.checkFirmware(event); err != nil {
			t.Fatal(err)
		}
		device, err := s.store.Device(id)
		if err != nil || device.FirmwareCheck == nil {
			t.Fatalf("check was not saved: %+v err=%v", device, err)
		}
		view := firmwareView(device)
		if strings.Contains(body, "Download failed") {
			if view.Problem == "" || !strings.Contains(recorder.Header().Get("Location"), url.QueryEscape("Download failed")) {
				t.Fatalf("check failure was hidden: %+v", view)
			}
			if _, err := s.devices.RefreshDevice(event.Request.Context(), id); err != nil {
				t.Fatal(err)
			}
			device, _ = s.store.Device(id)
			if firmwareView(device).Problem == "" {
				t.Fatal("status polling erased the update check error")
			}
		} else if view.Problem != "" || !view.Known || (body == `{}` && view.Available) {
			t.Fatalf("successful check lost availability, including an empty result: %+v", view)
		}
		if device.BaselineID != before.BaselineID || device.LastCaptured != before.LastCaptured {
			t.Fatal("firmware check changed configuration history")
		}
		snapshots, err := s.store.Snapshots(id)
		if err != nil || len(snapshots) != 1 {
			t.Fatalf("firmware check added snapshots: %d %v", len(snapshots), err)
		}
	}
}

func TestFirmwareManualCheckPrecedenceAndInvalidation(t *testing.T) {
	now := time.Now().UTC()
	device := domain.Device{Firmware: "2.1.0", Reachable: true, StatusAvailable: true, LastStatusRead: now,
		Status:        map[string]any{"sys": map[string]any{"available_updates": map[string]any{"stable": map[string]any{"version": "2.2.0"}}}},
		FirmwareCheck: &domain.FirmwareCheck{Firmware: "2.1.0", CheckedAt: now.Add(time.Second), Available: map[string]any{}}}
	if view := firmwareView(device); view.Available || !view.Known {
		t.Fatalf("fresh manual check lost to cached status: %+v", view)
	}
	device.LastStatusRead = now.Add(2 * time.Second)
	if view := firmwareView(device); view.Stable != "2.2.0" {
		t.Fatalf("new status lost to an older manual check: %+v", view)
	}
	device.Status["sys"].(map[string]any)["available_updates"].(map[string]any)["stable"].(map[string]any)["nsteps"] = float64(1)
	device.FirmwareCheck = &domain.FirmwareCheck{Firmware: "2.1.0", CheckedAt: now.Add(3 * time.Second), Available: map[string]any{"stable": map[string]any{"version": "2.2.0"}}}
	if view := firmwareView(device); !view.Staged {
		t.Fatalf("manual check hid the staged update reported by status: %+v", view)
	}
	device.FirmwareCheck.Problem = "Old check error"
	device = device.ApplyReading(domain.Reading{Identity: domain.Identity{Version: "2.2.0"}}, false, now.Add(3*time.Second))
	if device.FirmwareCheck != nil {
		t.Fatal("installed firmware change kept an obsolete check/error")
	}
}

func TestFirmwareRendersAccessibleIndicatorsAndCheckAction(t *testing.T) {
	device := domain.Device{ID: "device", DeviceID: "kitchen", Firmware: "2.1.0", Reachable: true, StatusAvailable: true,
		Status: map[string]any{"sys": map[string]any{"available_updates": map[string]any{"stable": map[string]any{"version": "2.2.0"}}}}}
	page := devicePage(PageMeta{CSRF: "test-csrf"}, domain.DeviceDetails{InventoryEntry: domain.InventoryEntry{Device: device}})
	templates, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := templates.ExecuteTemplate(&output, "detail", page); err != nil {
		t.Fatal(err)
	}
	body := output.String()
	for _, wanted := range []string{`id="firmware"`, `/devices/device/firmware/check`, `name="csrf" value="test-csrf"`, `#firmware`, "Check for updates", "Installed", "2.1.0", "2.2.0"} {
		if !strings.Contains(body, wanted) {
			t.Errorf("firmware detail missing %q", wanted)
		}
	}
	output.Reset()
	entry := domain.InventoryEntry{Device: device}
	inventory := inventoryPage(PageMeta{}, []domain.InventoryEntry{entry}, "", "", "", "")
	if inventory.Updates != 1 {
		t.Fatalf("pending updates not counted: %+v", inventory)
	}
	if err := templates.ExecuteTemplate(&output, "inventory", inventory); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `aria-label="Firmware for kitchen: Update available"`) {
		t.Fatal("inventory update indicator lost accessible label")
	}
}

func TestFirmwareProgressContinuesWhenTheUpdatingDeviceIsFilteredOut(t *testing.T) {
	entries := []domain.InventoryEntry{{Device: domain.Device{DeviceID: "updating", FirmwareUpdate: &domain.FirmwareUpdate{Phase: "verifying"}}}, {Device: domain.Device{DeviceID: "other"}}}
	templates, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}
	for _, active := range []bool{true, false} {
		if !active {
			entries[0].Device.FirmwareUpdate.Phase = "succeeded"
		}
		page := inventoryPage(PageMeta{}, entries, "other", "", "", "")
		var output bytes.Buffer
		if err := templates.ExecuteTemplate(&output, "inventory", page); err != nil {
			t.Fatal(err)
		}
		if len(page.Devices) != 1 || page.FirmwareRunning != active || strings.Contains(output.String(), "every 2s") != active {
			t.Fatalf("filtering lost running progress or completion: active=%v page=%+v", active, page)
		}
	}
}
