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
	}})
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
