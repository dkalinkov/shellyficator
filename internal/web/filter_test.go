package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"shellyficator/internal/domain"
)

func tagFilterTestServer(t *testing.T) (*testServer, string) {
	t.Helper()
	s, first := refreshTestServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) })
	fixtures := []struct{ name, room, tags, mac string }{
		{"Kitchen light", "Kitchen", "light, control", "AABBCCDDEEFF"},
		{"Kitchen fan", "Kitchen", "ventilator", "AABBCCDDEE01"},
		{"Hallway light", "Hallway", "light", "AABBCCDDEE02"},
		{"Desk LEDs", "Office", "led, ground floor", "AABBCCDDEE03"},
		{"Untyped", "Kitchen", "", "AABBCCDDEE04"},
	}
	for i, fixture := range fixtures {
		id := first
		if i > 0 {
			record, err := s.store.SaveReading(domain.Reading{Identity: domain.Identity{ID: fixture.name, Name: fixture.name, MAC: fixture.mac, Gen: 4}}, false)
			if err != nil {
				t.Fatal(err)
			}
			id = record.ID
		} else {
			record, _ := s.record(id)
			record.Set("name", fixture.name)
			if err := s.store.App.Save(record); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.store.Organization(id, fixture.room, fixture.tags); err != nil {
			t.Fatal(err)
		}
	}
	return s, first
}

func TestInventoryTagFilters(t *testing.T) {
	s, _ := tagFilterTestServer(t)
	for _, test := range []struct {
		name     string
		query    url.Values
		devices  []string
		selected []string
	}{
		{"no tags", nil, []string{"Desk LEDs", "Hallway light", "Kitchen fan", "Kitchen light", "Untyped"}, nil},
		{"one tag", url.Values{"tags": {"light"}}, []string{"Hallway light", "Kitchen light"}, []string{"light"}},
		{"any selected tag", url.Values{"tags": {"light, ventilator"}}, []string{"Hallway light", "Kitchen fan", "Kitchen light"}, []string{"light", "ventilator"}},
		{"repeated query values", url.Values{"tags": {"light", "ventilator"}}, []string{"Hallway light", "Kitchen fan", "Kitchen light"}, []string{"light", "ventilator"}},
		{"exact tag names", url.Values{"tags": {"li"}}, nil, []string{"li"}},
		{"normalize and deduplicate", url.Values{"tags": {" LIGHT, light, \n CONTROL ,,"}}, []string{"Hallway light", "Kitchen light"}, []string{"light", "control"}},
		{"multiword tag", url.Values{"tags": {" ground   floor "}}, []string{"Desk LEDs"}, []string{"ground floor"}},
		{"room intersection", url.Values{"room": {"Kitchen"}, "tags": {"light"}}, []string{"Kitchen light"}, []string{"light"}},
		{"search intersection", url.Values{"q": {"fan"}, "tags": {"light"}}, nil, []string{"light"}},
		{"search room and tags", url.Values{"q": {"fan"}, "room": {"Kitchen"}, "tags": {"light, ventilator"}}, []string{"Kitchen fan"}, []string{"light", "ventilator"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			event := &core.RequestEvent{App: s.store.App, Event: router.Event{Request: httptest.NewRequest("GET", "/?"+test.query.Encode(), nil), Response: response}}
			page, err := s.list(event)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, device := range page.Devices {
				names = append(names, device.Name)
			}
			sort.Strings(names)
			if !reflect.DeepEqual(names, test.devices) || !reflect.DeepEqual(page.SelectedTags, test.selected) {
				t.Fatalf("devices=%v selected=%v; want devices=%v selected=%v", names, page.SelectedTags, test.devices, test.selected)
			}
			if page.Total != 5 || page.Reachable != 5 {
				t.Fatalf("filters changed global inventory totals: %+v", page)
			}
			for _, tag := range []string{"control", "ground floor", "led", "light", "ventilator"} {
				if !strings.Contains(strings.Join(page.Tags, ","), tag) {
					t.Fatalf("filtered-out tag disappeared from choices: %q not in %v", tag, page.Tags)
				}
			}
		})
	}
}

func TestInventoryResponsePreservesTagFilterAndUpdatesChoices(t *testing.T) {
	s, _ := tagFilterTestServer(t)
	response := httptest.NewRecorder()
	event := &core.RequestEvent{App: s.store.App, Event: router.Event{Request: httptest.NewRequest("GET", "/inventory?tags=light", nil), Response: response}}
	if err := s.table(event); err != nil {
		t.Fatal(err)
	}
	if response.Header().Get("HX-Replace-Url") != "/?tags=light" {
		t.Fatalf("filter URL not preserved: %q", response.Header().Get("HX-Replace-Url"))
	}
	body := response.Body.String()
	if !strings.Contains(renderedText(body), "2 shown") || renderedAttribute(body, "select", "tag-filter-options", "hx-swap-oob") != "innerHTML" || !strings.Contains(body, `<option>ventilator</option>`) {
		t.Fatalf("missing filtered count or global tag choices: %s", body)
	}
}

func TestOrganizationSaveRetainsFilterAfterLastTagAssignmentIsRemoved(t *testing.T) {
	s, _ := tagFilterTestServer(t)
	records, _ := s.records()
	var id string
	for _, record := range records {
		if record.GetString("name") == "Desk LEDs" {
			id = record.Id
		}
	}
	form := url.Values{"room": {"Office"}, "tags": {"new category"}, "filter_q": {"Desk"}, "filter_room": {"Office"}, "filter_tags": {"ground floor"}}
	request := httptest.NewRequest("POST", "/devices/"+id+"/organization", strings.NewReader(form.Encode()))
	request.SetPathValue("id", id)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	event := &core.RequestEvent{App: s.store.App, Event: router.Event{Request: request, Response: response}}
	if err := s.organization(event); err != nil {
		t.Fatal(err)
	}
	location, _ := url.Parse(response.Header().Get("HX-Replace-Url"))
	if location.Path != "/" || location.Query().Get("tags") != "ground floor" || location.Query().Get("q") != "Desk" || location.Query().Get("room") != "Office" {
		t.Fatalf("active filters were lost on save: %s", location)
	}
	body := response.Body.String()
	if !strings.Contains(body, "No matching devices.") || !strings.Contains(body, "0 shown") || !strings.Contains(body, `<option>ground floor</option>`) || !strings.Contains(body, `<option>new category</option>`) {
		t.Fatalf("removed filter or new tag missing from refreshed choices: %s", body)
	}
}
