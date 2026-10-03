package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"shellyficator/internal/inventory"
)

func detailEvent(s *Server, id, method, action string, form url.Values) (*core.RequestEvent, *httptest.ResponseRecorder) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, "/devices/"+id+action, strings.NewReader(form.Encode()))
	request.SetPathValue("id", id)
	if method == "POST" {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return &core.RequestEvent{App: s.store.App, Event: router.Event{Request: request, Response: response}}, response
}

func TestDeviceDetailReadingStatesAndMappings(t *testing.T) {
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	record, _ := s.store.Device(id)
	record.Set("name", "Desk <light>")
	record.Set("status", map[string]any{
		"switch:3":  map[string]any{"output": true, "temperature": map[string]any{"tC": nil}, "apower": 105},
		"switch:10": map[string]any{"output": false},
		"input:9":   map[string]any{"state": nil}, "input:10": map[string]any{"state": false},
		"wifi":  map[string]any{"rssi": 0, "ssid": "home_IoT"},
		"sys":   map[string]any{"uptime": 0},
		"cloud": map[string]any{"connected": false},
	})
	record.Set("config", map[string]any{
		"switch:3":  map[string]any{"input_id": 9, "in_mode": "detached", "initial_state": "restore_last"},
		"switch:10": map[string]any{"in_mode": "follow"},
		"input:9":   map[string]any{"type": "button", "enable": false, "invert": false},
		"input:10":  map[string]any{"type": "switch", "enable": true},
	})
	baseline := inventory.Object(record, "config")
	view := detailDevice(record, baseline, nil, map[string]string{"input:9": "Physical switch note"})
	if len(view.Outputs) != 2 || len(view.Inputs) != 2 || view.Outputs[0].Key != "switch:3" || view.Outputs[1].Key != "switch:10" {
		t.Fatalf("incorrect grouping/order: %+v", view)
	}
	if view.Inputs[0].State != "Not reported" || view.Inputs[1].State != "Off" || view.Outputs[0].State != "On" || view.Outputs[1].State != "Off" {
		t.Fatalf("incorrect channel states: %+v", view)
	}
	if view.Inputs[0].Input.RelayKey != "switch:3" || view.Inputs[0].Input.ModeTone != "violet" || view.Inputs[0].Input.Changed || view.Inputs[0].Notes != "Physical switch note" {
		t.Fatalf("mapping or detached state: %+v", view.Inputs[0])
	}
	ssidReported := false
	for _, metric := range view.Network {
		ssidReported = ssidReported || (metric.Label == "Network" && metric.Value == "home_IoT")
	}
	if view.Outputs[0].Values[0].Value != "Not reported" || view.Summary[0].Value != "0" || view.Summary[1].Value != "0s" || !ssidReported {
		t.Fatalf("null/zero/identity changed: %+v", view)
	}
	for _, value := range view.Outputs[0].Values {
		if value.Label == "Power" {
			t.Fatal("power readings violate device page scope")
		}
	}

	for _, test := range []struct {
		name                      string
		reachable, status, config bool
		state, configuration      string
	}{
		{"online", true, true, true, "On", "Matches"},
		{"offline", false, false, false, "Unavailable", "Matches"},
		{"status failed", true, false, true, "Unavailable", "Matches"},
		{"config failed", true, true, false, "On", "Matches"},
	} {
		t.Run(test.name, func(t *testing.T) {
			record.Set("reachable", test.reachable)
			record.Set("status_available", test.status)
			record.Set("config_available", test.config)
			result := detailDevice(record, baseline, nil, nil)
			if result.Outputs[0].State != test.state || result.Table.Configuration != test.configuration {
				t.Fatalf("failures conflated: %+v", result)
			}
			if !test.status && (result.Outputs[0].StateTone != "muted" || result.Outputs[0].Values[0].Value != "Unavailable" || result.Summary[0].Tone != "muted") {
				t.Fatalf("cached status presented as current: %+v", result)
			}
			if !test.config && result.Table.ConfigurationTone != "muted" {
				t.Fatal("cached comparison presented as current")
			}
		})
	}
}

func TestDeviceDetailRendersFailureAndEmptyStates(t *testing.T) {
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	for _, test := range []struct {
		name   string
		mutate func(*core.Record)
		want   string
	}{
		{"ordinary", func(r *core.Record) { r.Set("name", "Kitchen <lamp>") }, "Kitchen &lt;lamp&gt;"},
		{"offline", func(r *core.Record) { r.Set("reachable", false) }, "Offline"},
		{"configuration failed", func(r *core.Record) { r.Set("reachable", true); r.Set("config_available", false) }, "Comparison unavailable"},
		{"missing baseline", func(r *core.Record) { r.Set("baseline", "") }, "Set approved baseline"},
		{"empty", func(r *core.Record) { r.Set("status", nil); r.Set("config", nil) }, "No channel data yet"},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, _ := s.store.Device(id)
			test.mutate(r)
			if err := s.store.App.Save(r); err != nil {
				t.Fatal(err)
			}
			event, response := detailEvent(s, id, "GET", "", nil)
			if err := s.detail(event); err != nil {
				t.Fatal(err)
			}
			body := response.Body.String()
			if response.Code != 200 || !strings.Contains(body, test.want) {
				t.Fatalf("render failed: %d %s", response.Code, body)
			}
			if test.name == "ordinary" && (!strings.Contains(body, `id="note-input:0"`) || !strings.Contains(body, `id="note-switch:0"`) || !strings.Contains(body, `name="csrf"`)) {
				t.Fatal("notes deep links or CSRF missing")
			}
		})
	}
}

func TestDeviceDetailRendersSavedSettingsWithoutMutingLiveChannels(t *testing.T) {
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	record, _ := s.store.Device(id)
	record.Set("status", map[string]any{
		"switch:0":      map[string]any{"output": false},
		"temperature:0": map[string]any{"tC": 0},
	})
	record.Set("config", map[string]any{
		"switch:0":      map[string]any{"initial_state": "off"},
		"temperature:0": map[string]any{"enable": false},
	})
	if err := s.store.App.Save(record); err != nil {
		t.Fatal(err)
	}
	if err := s.store.SaveChannelNote(id, "temperature:0", "Sensor note"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name            string
		configAvailable bool
	}{
		{"failed configuration", false},
		{"current configuration", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			record.Set("config_available", test.configAvailable)
			if err := s.store.App.Save(record); err != nil {
				t.Fatal(err)
			}
			event, response := detailEvent(s, id, "GET", "", nil)
			if err := s.detail(event); err != nil {
				t.Fatal(err)
			}
			body := response.Body.String()
			if response.Code != http.StatusOK || strings.Contains(body, "Settings unavailable · showing saved copy") == test.configAvailable {
				t.Fatal("configuration availability warning did not follow the latest read")
			}
			for _, channel := range []struct{ key, reading, setting, note string }{
				{"switch:0", "Off", "After power loss Off", "Add note"},
				{"temperature:0", "0 °C", "Enabled No", "Edit note"},
			} {
				var card string
				for _, fragment := range strings.Split(body, "<article ")[1:] {
					fragment = strings.SplitN(fragment, "</article>", 2)[0]
					if strings.Contains(fragment, `>`+channel.key+"</p>") {
						card = fragment
						break
					}
				}
				if card == "" || !strings.Contains(renderedText(card), channel.reading) || strings.Contains(card, "Unavailable") || !strings.Contains(renderedText(card), channel.setting) {
					t.Fatalf("%s lost its current reading or preserved setting: %s", channel.key, card)
				}
				if strings.Contains(card, "detail-saved-settings") == test.configAvailable {
					t.Fatalf("%s settings freshness differs from the configuration read", channel.key)
				}
				_, noteLink, _ := strings.Cut(card, `href="#note-`)
				href, _, _ := strings.Cut(noteLink, `"`)
				href = "#note-" + href
				anchor, err := url.PathUnescape(href)
				if err != nil || anchor != "#note-"+channel.key || !strings.Contains(noteLink, channel.note) || !strings.Contains(body, `id="note-`+channel.key+`"`) {
					t.Fatalf("%s note link lost its editor or saved-note state: %s", channel.key, noteLink)
				}
			}
		})
	}
}

func renderedText(markup string) string {
	tokens := html.NewTokenizer(strings.NewReader(markup))
	var text strings.Builder
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			return strings.Join(strings.Fields(text.String()), " ")
		case html.TextToken:
			text.Write(tokens.Text())
			text.WriteByte(' ')
		}
	}
}

func TestDeviceDetailReportedFaults(t *testing.T) {
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	record, _ := s.store.Device(id)
	record.Set("status", map[string]any{"switch:0": map[string]any{"output": false, "errors": []any{"overtemp"}}})
	view := detailDevice(record, nil, nil, nil)
	if view.Table.Connectivity != "Device fault" || len(view.Faults) != 1 || !strings.Contains(view.Faults[0], "overtemp") {
		t.Fatalf("fault lost: %+v", view)
	}
	record.Set("status_available", false)
	if view = detailDevice(record, nil, nil, nil); len(view.Faults) != 0 || view.Outputs[0].State != "Unavailable" {
		t.Fatalf("cached fault presented as current: %+v", view)
	}
}

func TestDeviceDetailKeepsSettingsWithTheirOwners(t *testing.T) {
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	record, _ := s.store.Device(id)
	record.Set("config", map[string]any{
		"input:9":  map[string]any{"type": "button", "enable": false, "invert": true},
		"switch:0": map[string]any{"input_id": 9, "in_mode": "detached", "in_locked": true, "initial_state": "restore_last"},
		"switch:1": map[string]any{"in_mode": "follow", "in_locked": true, "initial_state": "off"},
	})
	record.Set("status", map[string]any{
		"switch:0": map[string]any{"output": true, "tC": 18, "temperature": map[string]any{"tC": 22}},
		"switch:1": map[string]any{"output": false, "tC": 0},
	})
	view := detailDevice(record, nil, nil, nil)
	find := func(metrics []Metric, label string) (string, bool) {
		for _, metric := range metrics {
			if metric.Label == label {
				return metric.Value, true
			}
		}
		return "", false
	}
	input := view.Inputs[0].Input
	if input.RelayKey != "switch:0" || input.Mode != "Detached" {
		t.Fatalf("input lost mapped relay behavior: %+v", input)
	}
	if locked, _ := find(input.Values, "Locked"); locked != "Yes" {
		t.Fatal("input lock state was hidden")
	}
	if enabled, _ := find(input.Values, "Enabled"); enabled != "No" {
		t.Fatal("disabled input state was hidden")
	}
	if _, exists := find(input.Values, "After power loss"); exists {
		t.Fatal("relay power-loss setting is duplicated under the input")
	}
	if powerLoss, _ := find(view.Outputs[0].Settings, "After power loss"); powerLoss != "Restore last" {
		t.Fatal("relay lost its power-loss setting")
	}
	for _, label := range []string{"Input behavior", "Input locked"} {
		if _, exists := find(view.Outputs[0].Settings, label); exists {
			t.Fatalf("mapped input setting duplicated on relay: %s", label)
		}
		if _, exists := find(view.Outputs[1].Settings, label); !exists {
			t.Fatalf("unmapped relay setting lost: %s", label)
		}
	}
	if _, exists := find(view.Table.Inputs[0].Values, "After power loss"); !exists {
		t.Fatal("detail filtering modified the inventory presenter")
	}
	if len(view.Outputs[0].Values) != 1 || view.Outputs[0].Values[0].Value != "22 °C" || view.Outputs[1].Values[0].Value != "0 °C" {
		t.Fatalf("temperature aliases duplicated or zero reading lost: %+v", view.Outputs)
	}
}

func TestDeviceDetailShowsDistinctLastSeenTime(t *testing.T) {
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	record, _ := s.store.Device(id)
	record.Set("last_checked", "2026-10-02 17:00:00.500Z")
	for _, test := range []struct {
		name, lastSeen string
		wantSeen       bool
	}{
		{"same displayed time", "2026-10-02 17:00:00.400Z", false},
		{"older successful contact", "2026-10-01 17:00:00.000Z", true},
		{"never seen", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			record.Set("last_seen", test.lastSeen)
			view := detailDevice(record, nil, nil, nil)
			found := false
			for _, metric := range view.Network {
				if metric.Label == "Last seen" {
					found = true
					if metric.Value != inventory.FormatTime(test.lastSeen) {
						t.Fatal("last-seen value changed")
					}
				}
			}
			if found != test.wantSeen {
				t.Fatalf("last-seen visibility: got %v, want %v", found, test.wantSeen)
			}
		})
	}
}

func TestSelectedDeviceRefreshPreservesHistoryAndIdentity(t *testing.T) {
	for _, mode := range []string{"success", "status-failed", "identity-mismatch"} {
		t.Run(mode, func(t *testing.T) {
			s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("device mutation attempted: %s", r.Method)
				}
				switch r.URL.Path {
				case "/shelly":
					mac := "AABBCCDDEEFF"
					if mode == "identity-mismatch" {
						mac = "FFEEDDCCBBAA"
					}
					_, _ = w.Write([]byte(`{"id":"test-device","mac":"` + mac + `","gen":4}`))
				case "/rpc/Shelly.GetStatus":
					if mode == "status-failed" {
						w.WriteHeader(503)
						return
					}
					_, _ = w.Write([]byte(`{"switch:0":{"output":true},"input:0":{"state":false}}`))
				default:
					t.Errorf("unexpected request during status-only refresh: %s", r.URL.Path)
					w.WriteHeader(404)
				}
			})
			before, _ := s.store.Device(id)
			baselineID := before.GetString("baseline")
			config := before.GetString("config")
			captured := before.GetString("last_captured")
			event, response := detailEvent(s, id, "POST", "/refresh", nil)
			if err := s.refreshDevice(event); err != nil {
				t.Fatal(err)
			}
			after, _ := s.store.Device(id)
			snapshots, _ := s.store.Snapshots(id)
			if after.GetString("baseline") != baselineID || len(snapshots) != 1 || snapshots[0].Id != baselineID || after.GetString("config") != config || after.GetString("last_captured") != captured {
				t.Fatal("refresh changed configuration, baseline or snapshots")
			}
			if after.GetString("mac") != "AABBCCDDEEFF" {
				t.Fatal("identity mismatch rebound device")
			}
			if response.Code != http.StatusSeeOther {
				t.Fatalf("did not redirect: %d", response.Code)
			}
			if mode == "success" && (!after.GetBool("reachable") || !after.GetBool("status_available") || !inventory.Object(after, "status")["switch:0"].(map[string]any)["output"].(bool)) {
				t.Fatal("status was not updated")
			}
			if mode == "status-failed" && (!after.GetBool("reachable") || after.GetBool("status_available")) {
				t.Fatal("read failure mistaken for offline device")
			}
			if mode == "identity-mismatch" && after.GetBool("reachable") {
				t.Fatal("identity mismatch accepted as current status")
			}
		})
	}
}

func TestDetailNotesAndMetadataSaveIndependently(t *testing.T) {
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	metadata, _ := detailEvent(s, id, "POST", "/metadata", url.Values{"room": {"Office"}, "tags": {"lighting"}, "notes": {"Device note"}})
	if err := s.metadata(metadata); err != nil {
		t.Fatal(err)
	}
	channel, _ := detailEvent(s, id, "POST", "/channel-notes", url.Values{"component": {"input:0"}, "notes": {"Physical input note"}})
	if err := s.channelNote(channel); err != nil {
		t.Fatal(err)
	}
	record, _ := s.store.Device(id)
	if record.GetString("room") != "Office" || record.GetString("notes") != "Device note" || s.store.ChannelNotes(id)["input:0"] != "Physical input note" {
		t.Fatal("note save changed unrelated metadata")
	}
}
