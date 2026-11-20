package web

import (
	"net/url"
	"slices"
	"sort"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/domain"
)

type inventoryFilter struct {
	query, room string
	tags        []string
	tagSet      map[string]bool
}

func newInventoryFilter(query, room, tags string) inventoryFilter {
	filter := inventoryFilter{query: strings.TrimSpace(query), room: room, tags: domain.SplitTags(tags), tagSet: map[string]bool{}}
	for _, tag := range filter.tags {
		filter.tagSet[strings.ToLower(tag)] = true
	}
	return filter
}

func inventoryFilterFromQuery(query url.Values) inventoryFilter {
	return newInventoryFilter(query.Get("q"), query.Get("room"), strings.Join(query["tags"], ","))
}

func (f inventoryFilter) matches(device domain.Device) bool {
	if f.room != "" && f.room != device.Room {
		return false
	}
	if len(f.tagSet) > 0 && !slices.ContainsFunc(device.Tags, func(tag string) bool { return f.tagSet[strings.ToLower(tag)] }) {
		return false
	}
	if f.query == "" {
		return true
	}
	haystack := strings.Join([]string{deviceName(device), device.DeviceID, device.Address, device.Room, strings.Join(device.Tags, ", "), device.Notes}, " ")
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(f.query))
}

func (s *Server) list(e *core.RequestEvent) (InventoryPage, error) {
	query := e.Request.URL.Query()
	return s.listFiltered(e, inventoryFilterFromQuery(query), query.Get("group"))
}

func (s *Server) listFiltered(e *core.RequestEvent, filter inventoryFilter, groupBy string) (InventoryPage, error) {
	entries, err := s.devices.Inventory()
	if err != nil {
		return InventoryPage{}, err
	}
	preferences, err := s.watchPreferences()
	if err != nil {
		return InventoryPage{}, err
	}
	page := inventoryPage(s.pageMeta(e, "Devices"), entries, filter, groupBy, preferences)
	page.Discovery = s.devices.Discovery()
	return page, nil
}

func inventoryPage(meta PageMeta, entries []domain.InventoryEntry, filter inventoryFilter, groupBy string, watch domain.WatchPreferences) InventoryPage {
	if groupBy != "room" {
		groupBy = ""
	}
	page := InventoryPage{PageMeta: meta, Query: filter.query, Room: filter.room, SelectedTags: slices.Clone(filter.tags), GroupBy: groupBy}
	rooms := map[string]bool{}
	tags := map[string]string{}
	page.Total = len(entries)
	for _, entry := range entries {
		model := entry.Device
		page.FirmwareRunning = page.FirmwareRunning || model.FirmwareUpdate.Active()
		if model.Reachable {
			page.Reachable++
		}
		if !model.AuthEnabled {
			page.Unprotected++
		}
		firmware := firmwareView(model)
		if firmware.Current && firmware.Stable != "" {
			page.Updates++
		}
		comparison := domain.CompareConfiguration(entry.Baseline, model.Config, watch.ForDevice(model.ID))
		if len(comparison.Settings) > 0 {
			page.Changed++
		}
		room := model.Room
		if room != "" {
			rooms[room] = true
		}
		for _, tag := range model.Tags {
			key := strings.ToLower(tag)
			if _, known := tags[key]; !known {
				tags[key] = tag
			}
		}
		if !filter.matches(model) {
			continue
		}
		device := Device{Info: deviceView(model), Name: deviceName(model), Table: tableDevice(model, entry.Baseline, comparison, firmware)}
		notes := entry.ChannelNotes
		for i := range device.Table.Inputs {
			input := &device.Table.Inputs[i]
			input.Note = previewText(notes[input.Key], 320)
			input.RelayNote = previewText(notes[input.RelayKey], 320)
		}
		page.Devices = append(page.Devices, device)
	}
	for room := range rooms {
		page.Rooms = append(page.Rooms, room)
	}
	// Keep an active filter available even after its last device moves rooms.
	if page.Room != "" && !rooms[page.Room] {
		page.Rooms = append(page.Rooms, page.Room)
	}
	sort.Strings(page.Rooms)
	// Preserve active tags even when an edit removes their last assignment.
	for i, tag := range page.SelectedTags {
		key := strings.ToLower(tag)
		if name, known := tags[key]; known {
			page.SelectedTags[i] = name
		} else {
			tags[key] = tag
		}
	}
	for _, tag := range tags {
		page.Tags = append(page.Tags, tag)
	}
	sort.Slice(page.Tags, func(i, j int) bool { return strings.ToLower(page.Tags[i]) < strings.ToLower(page.Tags[j]) })
	if page.GroupBy == "room" {
		page.RoomGroups = groupDevicesByRoom(page.Devices)
	}
	return page
}

func groupDevicesByRoom(devices []Device) []RoomGroup {
	var groups []RoomGroup
	indices := map[string]int{}
	for _, device := range devices {
		room := device.Info.Room
		index, found := indices[room]
		if !found {
			index = len(groups)
			indices[room] = index
			groups = append(groups, RoomGroup{Room: room})
		}
		groups[index].Devices = append(groups[index].Devices, device)
	}
	sort.Slice(groups, func(i, j int) bool {
		a, b := groups[i].Room, groups[j].Room
		if a == "" {
			return false
		}
		if b == "" {
			return true
		}
		if strings.ToLower(a) == strings.ToLower(b) {
			return a < b
		}
		return strings.ToLower(a) < strings.ToLower(b)
	})
	for i := range groups {
		groups[i].Index = i
	}
	return groups
}

func inventoryURL(page InventoryPage) string {
	query := url.Values{}
	if page.Query != "" {
		query.Set("q", page.Query)
	}
	if page.Room != "" {
		query.Set("room", page.Room)
	}
	if len(page.SelectedTags) > 0 {
		query.Set("tags", strings.Join(page.SelectedTags, ","))
	}
	if page.GroupBy != "" {
		query.Set("group", page.GroupBy)
	}
	if len(query) == 0 {
		return "/"
	}
	return "/?" + query.Encode()
}

func (s *Server) dashboard(e *core.RequestEvent) error {
	page, err := s.list(e)
	if err != nil {
		return err
	}
	page.RememberedNetworks, _ = s.devices.DiscoveryNetworks()
	networks, _ := s.devices.LocalNetworks()
	for _, network := range networks {
		if !slices.Contains(page.RememberedNetworks, network.Prefix) {
			page.Networks = append(page.Networks, network)
		}
	}
	return s.render(e, "dashboard", page)
}

func (s *Server) forgetNetwork(e *core.RequestEvent) error {
	network := domain.NetworkPrefix(e.Request.FormValue("network"))
	if network == "" || s.devices.ForgetNetwork(network) != nil {
		return s.redirect(e, "/", "Could not forget that network.", "error")
	}
	return s.redirect(e, "/", "Discover no longer checks "+network+".")
}

func (s *Server) progress(e *core.RequestEvent) error {
	return s.render(e, "discovery", DiscoveryPage{Discovery: s.devices.Discovery()})
}

func (s *Server) discover(e *core.RequestEvent) error {
	if err := s.devices.Discover(e.Request.FormValue("target"), e.Request.FormValue("mode") == "auto"); err != nil {
		return s.redirect(e, "/", err.Error(), "error")
	}
	return s.redirect(e, "/", "")
}

func (s *Server) refreshInventory(e *core.RequestEvent) error {
	force := e.Request.URL.Query().Get("refresh") == "1"
	done := s.devices.Refresh(force)
	if done != nil {
		select {
		case <-done:
		case <-e.Request.Context().Done():
			return e.Request.Context().Err()
		}
	}
	return s.table(e)
}

func (s *Server) table(e *core.RequestEvent) error {
	page, err := s.list(e)
	if err != nil {
		return err
	}
	e.Response.Header().Set("HX-Replace-Url", s.base(e)+inventoryURL(page))
	return s.render(e, "inventory-updated", page)
}
