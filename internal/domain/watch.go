package domain

import "strings"

type WatchGroup string

const (
	WatchButtons      WatchGroup = "buttons"
	WatchPower        WatchGroup = "power"
	WatchTimers       WatchGroup = "timers"
	WatchAutomations  WatchGroup = "automations"
	WatchConnectivity WatchGroup = "connectivity"
	WatchOther        WatchGroup = "other"
	WatchMetadata     WatchGroup = "metadata"
)

var WatchGroups = []WatchGroup{WatchButtons, WatchPower, WatchTimers, WatchAutomations, WatchConnectivity, WatchOther, WatchMetadata}

// A nil policy means defaults; an empty policy deliberately records everything only.
type WatchPolicy map[WatchGroup]bool

func (p WatchPolicy) Warn(group WatchGroup) bool {
	if p == nil {
		return group == WatchButtons || group == WatchPower
	}
	return p[group]
}

// WatchPreferences contains an app-wide default and optional device overrides.
// Changing it never changes device configuration or approved baselines.
type WatchPreferences map[string]WatchPolicy

func (p WatchPreferences) ForDevice(id string) WatchPolicy {
	if policy, ok := p[id]; id != "" && ok {
		return policy
	}
	return p[""]
}

func ConfigurationGroup(path string) WatchGroup {
	if configurationMetadata(path) {
		return WatchMetadata
	}
	parts := strings.Split(path, " / ")
	kind, _, numbered := strings.Cut(parts[0], ":")
	field := ""
	if len(parts) > 1 {
		field = parts[1]
	}
	if numbered && kind == "input" && field != "name" {
		return WatchButtons
	}
	if path == "sys / device / profile" || path == "sys / device / sys_btn_toggle" {
		return WatchButtons
	}
	if numbered {
		switch kind {
		case "switch", "cover", "light", "rgb", "rgbw", "cct", "rgbcct":
			switch field {
			case "input_id", "in_mode", "in_locked", "in_enabled", "in_invert", "swap_inputs", "button_presets":
				return WatchButtons
			case "initial_state", "power_restore_delay", "power_restore_delay_enable":
				return WatchPower
			case "auto_on", "auto_off", "auto_on_delay", "auto_off_delay":
				return WatchTimers
			}
		}
		if kind == "script" {
			return WatchAutomations
		}
	}
	switch parts[0] {
	case "wifi", "eth", "mqtt", "cloud", "ws", "ble", "bthome", "matter", "zigbee", "http", "rpc":
		return WatchConnectivity
	}
	if strings.HasPrefix(path, "sys / rpc_udp") || path == "sys / device / enhanced_security" || path == "sys / device / tls_check_cert_validity_time" {
		return WatchConnectivity
	}
	return WatchOther
}
