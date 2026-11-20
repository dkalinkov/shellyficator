package web

import (
	"errors"
	"net/url"

	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
)

type watchGroupView struct {
	ID                 domain.WatchGroup
	Label, Description string
	Warn               bool
}

type settingsPage struct {
	PageMeta
	DeviceID, DeviceName string
	Inherit              bool
	Groups               []watchGroupView
}

func (s *Server) watchPreferences() (domain.WatchPreferences, error) {
	if s.watchStore == nil {
		return nil, nil
	}
	return s.watchStore.WatchPreferences()
}

func (s *Server) settingsPage(e *core.RequestEvent) error {
	preferences, err := s.watchPreferences()
	if err != nil {
		return err
	}
	id := e.Request.URL.Query().Get("device")
	page := settingsPage{PageMeta: s.pageMeta(e, "Settings"), DeviceID: id}
	if id != "" {
		device, err := s.devices.Device(id)
		if errors.Is(err, domain.ErrNotFound) {
			return e.NotFoundError("Device not found", nil)
		}
		if err != nil {
			return err
		}
		page.DeviceName = deviceName(device)
		_, overridden := preferences[id]
		page.Inherit = !overridden
	}
	policy := preferences.ForDevice(id)
	page.Groups = []watchGroupView{
		{ID: domain.WatchButtons, Label: "Button behavior", Description: "Input mode, enabled state, inversion, relay binding, input locks, button presets, and device profile."},
		{ID: domain.WatchPower, Label: "Behavior after power loss", Description: "Output state on startup and power restoration delays."},
		{ID: domain.WatchTimers, Label: "Timers", Description: "Automatic on/off settings and their delays."},
		{ID: domain.WatchAutomations, Label: "Script settings", Description: "Script component settings, such as name and start-on-boot, when reported by the device. Script code and running state are not compared."},
		{ID: domain.WatchConnectivity, Label: "Connectivity and access", Description: "Reported Wi-Fi, Ethernet, cloud, MQTT, Bluetooth, and access configuration. Password values are not compared."},
		{ID: domain.WatchOther, Label: "Other settings", Description: "All remaining settings, including device names, protection limits, location, and unrecognized fields."},
		{ID: domain.WatchMetadata, Label: "Device metadata", Description: "Configuration revision counters, firmware build identifiers, base MAC addresses, and component IDs."},
	}
	for i := range page.Groups {
		page.Groups[i].Warn = policy.Warn(page.Groups[i].ID)
	}
	return s.render(e, "settings", page)
}

func (s *Server) saveSettings(e *core.RequestEvent) error {
	if s.watchStore == nil {
		return e.InternalServerError("Settings storage unavailable", nil)
	}
	if err := e.Request.ParseForm(); err != nil {
		return e.BadRequestError("Invalid settings form", err)
	}
	id := e.Request.PostForm.Get("device")
	policy := domain.WatchPolicy{}
	for _, group := range domain.WatchGroups {
		values := e.Request.PostForm[string(group)]
		if len(values) != 1 || (values[0] != "warn" && values[0] != "record") {
			return e.BadRequestError("Choose Warn me or Record only for every group", nil)
		}
		policy[group] = values[0] == "warn"
	}
	if id != "" && e.Request.PostForm.Get("inherit") == "true" {
		policy = nil
	}
	if err := s.watchStore.SaveWatchPolicy(id, policy); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return e.NotFoundError("Device not found", nil)
		}
		return e.InternalServerError("Could not save change preferences", err)
	}
	query := url.Values{"notice": {"Change preferences saved. Device settings and baselines are unchanged."}}
	if id != "" {
		query.Set("device", id)
	}
	return s.redirect(e, "/settings?"+query.Encode(), "")
}
