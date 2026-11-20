package web

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"shellyficator/internal/domain"
)

func TestCloudConnectionPresentation(t *testing.T) {
	for _, test := range []struct {
		name                          string
		status, config                map[string]any
		reachable, statusOK, configOK bool
		value, tone                   string
	}{
		{"connected", map[string]any{"connected": true}, map[string]any{"enable": true}, true, true, true, "Connected", "ok"},
		{"enabled but disconnected", map[string]any{"connected": false}, map[string]any{"enable": true}, true, true, true, "Disconnected", "warning"},
		{"disabled", map[string]any{"connected": false}, map[string]any{"enable": false}, true, true, true, "Disabled", "muted"},
		{"connection supersedes saved disabled setting", map[string]any{"connected": true}, map[string]any{"enable": false}, true, true, true, "Connected", "ok"},
		{"configuration failure keeps connection", map[string]any{"connected": true}, nil, true, true, false, "Connected", "ok"},
		{"failed config cannot claim disabled", map[string]any{"connected": false}, map[string]any{"enable": false}, true, true, false, "Disconnected", "warning"},
		{"status failure retains cached connected value", map[string]any{"connected": true}, map[string]any{"enable": true}, true, false, true, "Connected", "muted"},
		{"status failure retains cached disabled value", map[string]any{"connected": false}, map[string]any{"enable": false}, true, false, true, "Disabled", "muted"},
		{"offline retains cached connected value", map[string]any{"connected": true}, map[string]any{"enable": true}, false, true, true, "Connected", "muted"},
		{"missing cloud", nil, nil, true, true, true, "Unknown", "muted"},
		{"null state", map[string]any{"connected": nil}, map[string]any{"enable": true}, true, true, true, "Unknown", "muted"},
		{"malformed state", map[string]any{"connected": "false"}, map[string]any{"enable": true}, true, true, true, "Unknown", "muted"},
		{"disabled with no connection reading", nil, map[string]any{"enable": false}, true, true, true, "Disabled", "muted"},
	} {
		t.Run(test.name, func(t *testing.T) {
			device := domain.Device{
				Status: map[string]any{"cloud": test.status}, Config: map[string]any{"cloud": test.config},
				Reachable: test.reachable, StatusAvailable: test.statusOK, ConfigAvailable: test.configOK,
			}
			detail := detailDevice(device, nil, domain.ConfigurationComparison{}, nil)
			cloud := detail.Table.Cloud
			if cloud.Value != test.value || cloud.Tone != test.tone {
				t.Fatalf("incorrect cloud state in the inventory: %+v", cloud)
			}
			panel := slices.IndexFunc(detail.Network, func(field ConnectionField) bool { return field.Label == "Shelly Cloud" })
			if panel < 0 || detail.Network[panel].Value != test.value || detail.Network[panel].Tone != test.tone {
				t.Fatalf("incorrect cloud state in the connection panel: %+v", detail.Network)
			}
			if (!test.reachable || !test.statusOK) && !strings.Contains(cloud.Info, "Last reported") {
				t.Fatal("cached cloud connection lost its freshness label")
			}
		})
	}
}

func TestInventoryCloudIndicatorLinksToConnectionPanel(t *testing.T) {
	templates, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}
	for _, connected := range []bool{true, false} {
		device := domain.Device{
			ID: "lamp", Name: "Desk lamp", Reachable: true, StatusAvailable: true,
			Status: map[string]any{"cloud": map[string]any{"connected": connected}},
		}
		row := Device{Info: deviceView(device), Name: device.Name, Table: detailDevice(device, nil, domain.ConfigurationComparison{}, nil).Table}
		var body bytes.Buffer
		if err := templates.ExecuteTemplate(&body, "inventory-table", []Device{row}); err != nil {
			t.Fatal(err)
		}
		html := body.String()
		if strings.Contains(html, "cloud-indicator") != connected {
			t.Fatalf("unexpected cloud indicator for connected=%t", connected)
		}
		if connected && (!strings.Contains(html, `href="/devices/lamp#detail-connection"`) || !strings.Contains(html, `aria-label="Shelly Cloud: Connected for Desk lamp"`)) {
			t.Fatal("cloud indicator lost its destination or accessible name")
		}
	}
}

func TestConnectionFieldsUseIndependentReadStates(t *testing.T) {
	device := domain.Device{
		Reachable: true, StatusAvailable: true, ConfigAvailable: true,
		Status: map[string]any{
			"mqtt": map[string]any{"connected": false},
			"wifi": map[string]any{"status": "got ip", "ssid": "Home IoT"},
			"sys":  map[string]any{"restart_required": true},
		},
		Config: map[string]any{"mqtt": map[string]any{"enable": false}},
	}
	view := detailDevice(device, nil, domain.ConfigurationComparison{}, nil)
	fields := map[string]ConnectionField{}
	for _, field := range view.Network {
		fields[field.Label] = field
	}
	if fields["Wi-Fi"].Value != "Connected" || fields["MQTT"].Value != "Disabled" || fields["Network"].Value != "Home IoT" || fields["Restart"].Value != "Required" {
		t.Fatalf("connection fields lost reported states: %+v", fields)
	}
	device.StatusAvailable = false
	for _, field := range detailDevice(device, nil, domain.ConfigurationComparison{}, nil).Network {
		switch field.Label {
		case "Wi-Fi", "MQTT", "Network", "Restart":
			fields[field.Label] = field
		}
	}
	for _, label := range []string{"Wi-Fi", "MQTT", "Network", "Restart"} {
		if field := fields[label]; field.Value != "Unavailable" || field.Tone != "muted" {
			t.Fatalf("stale %s value shown as current: %+v", label, field)
		}
	}
	for _, state := range []struct{ reported, label, tone string }{{"connecting", "Connecting", "blue"}, {"disconnected", "Disconnected", "error"}} {
		wifi := wifiConnection(state.reported, true)
		if wifi.Value != state.label || wifi.Tone != state.tone {
			t.Fatalf("wrong Wi-Fi state: %+v", wifi)
		}
	}
}
