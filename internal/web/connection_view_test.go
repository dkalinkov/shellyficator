package web

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"shellyficator/internal/inventory"
)

func TestCloudConnectionPresentation(t *testing.T) {
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	record, _ := s.store.Device(id)
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
			record.Set("status", map[string]any{"cloud": test.status})
			record.Set("config", map[string]any{"cloud": test.config})
			record.Set("reachable", test.reachable)
			record.Set("status_available", test.statusOK)
			record.Set("config_available", test.configOK)
			detail := detailDevice(record, nil, nil, nil)
			cloud := detail.Table.Cloud
			if cloud.Value != test.value || cloud.Tone != test.tone || detail.Network[0].Value != cloud.Value || detail.Network[0].Tone != cloud.Tone {
				t.Fatalf("cloud state differs or is stale: table=%+v panel=%+v", cloud, detail.Network)
			}
			if (!test.reachable || !test.statusOK) && !strings.Contains(cloud.Info, "Last reported") {
				t.Fatal("cached cloud connection lost its freshness label")
			}
			page := DevicePage{Device: record, Name: "Desk <lamp>", Detail: detail}
			for _, name := range []string{"detail", "inventory-table"} {
				var body bytes.Buffer
				data := any(page)
				if name == "inventory-table" {
					data = []Device{{Record: record, Name: page.Name, Table: detail.Table}}
				}
				if err := s.templates.ExecuteTemplate(&body, name, data); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(body.String(), "Desk <lamp>") {
					t.Fatalf("%s lost name escaping", name)
				}
				if name == "detail" {
					if !strings.Contains(body.String(), "tone-"+test.tone) || !strings.Contains(body.String(), test.value) {
						t.Fatal("connection panel lost state or color")
					}
				} else {
					visible := test.value == "Connected"
					if strings.Contains(body.String(), "cloud-indicator") != visible {
						t.Fatal("inventory cloud visibility does not match confirmed connection")
					}
					if visible && (!strings.Contains(body.String(), "#detail-connection") || !strings.Contains(body.String(), "Shelly Cloud: Connected")) {
						t.Fatal("connected cloud icon lost its link or accessible state")
					}
				}
			}
		})
	}
}

func TestConnectionFieldsUseIndependentReadStates(t *testing.T) {
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	record, _ := s.store.Device(id)
	record.Set("status", map[string]any{
		"cloud": map[string]any{"connected": true},
		"mqtt":  map[string]any{"connected": false},
		"wifi":  map[string]any{"status": "got ip", "ssid": "Home IoT"},
		"sys":   map[string]any{"restart_required": true},
	})
	record.Set("config", map[string]any{"mqtt": map[string]any{"enable": false}})
	view := detailDevice(record, nil, nil, nil)
	fields := map[string]ConnectionField{}
	for _, field := range view.Network {
		fields[field.Label] = field
	}
	if fields["Wi-Fi"].Value != "Connected" || fields["Wi-Fi"].Tone != "ok" || fields["MQTT"].Value != "Disabled" || fields["Network"].Tone != "blue" || fields["Restart"].Tone != "warning" {
		t.Fatalf("connection fields lost reported states: %+v", fields)
	}
	record.Set("status_available", false)
	for _, field := range detailDevice(record, nil, nil, nil).Network {
		if field.Label == "Shelly Cloud" {
			if field.Value != "Connected" || field.Tone != "muted" || !strings.Contains(field.Info, "Last reported") {
				t.Fatalf("cached cloud connection is not labelled: %+v", field)
			}
			continue
		}
		if field.Value != "Unavailable" || field.Tone != "muted" {
			t.Fatalf("stale connection field shown as current: %+v", field)
		}
	}
	for _, state := range []struct{ value, tone string }{{"connected", "ok"}, {"connecting", "blue"}, {"disconnected", "error"}} {
		wifi := wifiConnection(state.value, true)
		if wifi.Tone != state.tone {
			t.Fatalf("wrong Wi-Fi state tone: %+v", wifi)
		}
	}
	if inventory.Object(record, "config")["mqtt"] == nil {
		t.Fatal("presenter mutated saved configuration")
	}
}
