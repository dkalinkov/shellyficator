package web

import (
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

func TestInventoryRoomGrouping(t *testing.T) {
	s, _ := tagFilterTestServer(t)
	records, err := s.store.Devices()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.GetString("name") == "Untyped" {
			if err := s.store.Organization(record.Id, "", ""); err != nil {
				t.Fatal(err)
			}
		}
		if record.GetString("name") == "Desk LEDs" {
			if err := s.store.Organization(record.Id, "attic", record.GetString("tags")); err != nil {
				t.Fatal(err)
			}
		}
	}

	type group struct {
		room    string
		devices []string
	}
	for _, test := range []struct {
		name   string
		query  string
		groups []group
	}{
		{"default flat list", "", nil},
		{"unsupported grouping", "group=tags", nil},
		{"rooms alphabetically and unassigned last", "group=room", []group{
			{"attic", []string{"Desk LEDs"}}, {"Hallway", []string{"Hallway light"}},
			{"Kitchen", []string{"Kitchen fan", "Kitchen light"}}, {"", []string{"Untyped"}},
		}},
		{"tag filter before grouping", "group=room&tags=light", []group{
			{"Hallway", []string{"Hallway light"}}, {"Kitchen", []string{"Kitchen light"}},
		}},
		{"all filters before grouping", "group=room&tags=light,ventilator&room=Kitchen&q=fan", []group{
			{"Kitchen", []string{"Kitchen fan"}},
		}},
		{"no matches means no empty groups", "group=room&q=missing", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			event := &core.RequestEvent{App: s.store.App, Event: router.Event{Request: httptest.NewRequest("GET", "/?"+test.query, nil), Response: httptest.NewRecorder()}}
			page, err := s.list(event)
			if err != nil {
				t.Fatal(err)
			}
			var got []group
			for _, section := range page.RoomGroups {
				entry := group{room: section.Room}
				for _, device := range section.Devices {
					entry.devices = append(entry.devices, device.Name)
				}
				got = append(got, entry)
			}
			if !reflect.DeepEqual(got, test.groups) {
				t.Fatalf("groups=%v; want %v", got, test.groups)
			}
			if page.Total != 5 || page.Reachable != 5 {
				t.Fatalf("grouping changed inventory totals: total=%d reachable=%d", page.Total, page.Reachable)
			}
			location, err := url.Parse(inventoryURL(page))
			if err != nil {
				t.Fatal(err)
			}
			wantGroup := ""
			if strings.Contains(test.query, "group=room") {
				wantGroup = "room"
			}
			if page.GroupBy != wantGroup || location.Query().Get("group") != wantGroup {
				t.Fatalf("grouping state=%q URL=%s; want %q", page.GroupBy, location, wantGroup)
			}
		})
	}
}

func TestInventoryGroupedResponsePreservesFilters(t *testing.T) {
	s, _ := tagFilterTestServer(t)
	response := httptest.NewRecorder()
	event := &core.RequestEvent{App: s.store.App, Event: router.Event{Request: httptest.NewRequest("GET", "/inventory?group=room&tags=light&q=light", nil), Response: response}}
	if err := s.table(event); err != nil {
		t.Fatal(err)
	}
	location, err := url.Parse(response.Header().Get("HX-Replace-Url"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Query().Get("group") != "room" || location.Query().Get("tags") != "light" || location.Query().Get("q") != "light" {
		t.Fatalf("grouping or filters lost: %s", location)
	}
	body := response.Body.String()
	singularCounts := regexp.MustCompile(`\b1 device\b`).FindAllString(renderedText(body), -1)
	if strings.Count(body, `data-room-group="`) != 2 || len(singularCounts) != 2 || !strings.Contains(renderedText(body), "2 shown") {
		t.Fatalf("missing room sections or filtered counts: %s", body)
	}
	if strings.Index(body, `data-room-group="Hallway"`) > strings.Index(body, `data-room-group="Kitchen"`) || strings.Contains(body, `data-room-group="Office"`) {
		t.Fatalf("incorrect room order or an empty group: %s", body)
	}
}

func TestOrganizationSavePreservesGroupingAndMovesDevice(t *testing.T) {
	s, _ := tagFilterTestServer(t)
	records, err := s.store.Devices()
	if err != nil {
		t.Fatal(err)
	}
	var id string
	for _, record := range records {
		if record.GetString("name") == "Desk LEDs" {
			id = record.Id
		}
	}
	form := url.Values{"room": {"Kitchen"}, "tags": {"led"}, "filter_q": {"Desk"}, "filter_tags": {"led"}, "filter_group": {"room"}}
	request := httptest.NewRequest("POST", "/devices/"+id+"/organization", strings.NewReader(form.Encode()))
	request.SetPathValue("id", id)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	event := &core.RequestEvent{App: s.store.App, Event: router.Event{Request: request, Response: response}}
	if err := s.organization(event); err != nil {
		t.Fatal(err)
	}
	location, err := url.Parse(response.Header().Get("HX-Replace-Url"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Query().Get("group") != "room" || location.Query().Get("tags") != "led" || location.Query().Get("q") != "Desk" {
		t.Fatalf("grouping or filters lost after save: %s", location)
	}
	body := response.Body.String()
	if strings.Count(body, `data-room-group="`) != 1 || !strings.Contains(body, `data-room-group="Kitchen"`) || strings.Contains(body, `data-room-group="Office"`) || !strings.Contains(body, "Desk LEDs") {
		t.Fatalf("device did not move to its new room group: %s", body)
	}
}
