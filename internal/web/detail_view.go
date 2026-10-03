package web

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/inventory"
)

type DetailChannel struct {
	Key, Name, Kind, Icon, Notes, State, StateTone string
	Values, Settings                               []Metric
	Input                                          *TableInput
	SavedSettings                                  bool
}

type DetailSummary struct {
	Label, Value, Unit, Tone, Icon string
}

type ChannelNoteEditor struct {
	Channel        DetailChannel
	DeviceID, CSRF string
}

type DeviceDetail struct {
	Table                            DeviceTable
	Outputs, Inputs, Other           []DetailChannel
	Summary                          []DetailSummary
	Network                          []ConnectionField
	Identity                         []Metric
	Faults                           []string
	Notes                            []ChannelNoteEditor
	StatusAvailable, ConfigAvailable bool
}

func detailDevice(record *core.Record, baseline map[string]any, differences []inventory.Difference, notes map[string]string) DeviceDetail {
	status, config := inventory.Object(record, "status"), inventory.Object(record, "config")
	view := DeviceDetail{
		Table:           tableDevice(record, baseline, len(differences)),
		StatusAvailable: inventory.StatusCurrent(record),
		ConfigAvailable: inventory.ConfigCurrent(record),
	}
	wifi, _ := status["wifi"].(map[string]any)
	sys, _ := status["sys"].(map[string]any)
	signal := DetailSummary{Label: "Wi-Fi signal", Value: "Not reported", Tone: "muted", Icon: "wifi"}
	if view.Table.Signal != "" {
		signal.Value = view.Table.SignalValue
		signal.Unit = "dBm"
		signal.Tone = view.Table.SignalTone
	}
	uptime := DetailSummary{Label: "Uptime", Value: detailDuration(sys["uptime"]), Tone: "muted", Icon: "clock"}
	if !view.StatusAvailable {
		signal.Value = "Unavailable"
		signal.Unit = ""
		signal.Tone = "muted"
		uptime.Value = "Unavailable"
	}
	security := DetailSummary{Label: "Device password", Value: "Not enabled", Tone: "warning", Icon: "unlock"}
	if record.GetBool("auth_enabled") {
		security.Value = "Enabled"
		security.Tone = "ok"
		security.Icon = "lock"
	}
	view.Summary = []DetailSummary{signal, uptime, security}
	view.Network = append(view.Network, ConnectionField{Label: "Shelly Cloud", Value: view.Table.Cloud.Value, Tone: view.Table.Cloud.Tone, Icon: "cloud", Info: view.Table.Cloud.Info, State: true})
	if value, exists := wifi["status"]; exists {
		connection := wifiConnection(value, view.StatusAvailable)
		view.Network = append(view.Network, ConnectionField{Label: "Wi-Fi", Value: connection.Value, Tone: connection.Tone, Icon: "wifi", State: true})
	}
	if value, exists := wifi["ssid"]; exists {
		tone := "muted"
		if view.StatusAvailable {
			if ssid, ok := value.(string); ok && ssid != "" {
				tone = "blue"
			}
		}
		view.Network = append(view.Network, ConnectionField{Label: "Network", Value: detailReading(value, view.StatusAvailable, ""), Tone: tone, Icon: "wifi"})
	}
	mqttStatus, statusReported := status["mqtt"].(map[string]any)
	mqttConfig, configReported := config["mqtt"].(map[string]any)
	if statusReported || configReported {
		connection := serviceConnection(mqttStatus, mqttConfig, view.StatusAvailable, view.ConfigAvailable)
		view.Network = append(view.Network, ConnectionField{Label: "MQTT", Value: connection.Value, Tone: connection.Tone, Icon: "network", Info: connection.Info, State: true})
	}
	if required, ok := sys["restart_required"].(bool); ok && (required || !view.StatusAvailable) {
		value, tone := "Required", "warning"
		if !view.StatusAvailable {
			value, tone = "Unavailable", "muted"
		}
		view.Network = append(view.Network, ConnectionField{Label: "Restart", Value: value, Tone: tone, Icon: "refresh", State: true})
	}
	if seen := record.GetString("last_seen"); seen != "" && inventory.FormatTime(seen) != inventory.FormatTime(record.GetString("last_checked")) {
		view.Network = append(view.Network, ConnectionField{Label: "Last seen", Value: inventory.FormatTime(seen), Tone: "muted", Icon: "clock"})
	}
	for _, field := range []struct{ key, label string }{{"device_id", "Device ID"}, {"mac", "MAC address"}, {"firmware", "Firmware"}, {"address", "Address"}} {
		value := record.GetString(field.key)
		if value == "" {
			value = "Not reported"
		}
		view.Identity = append(view.Identity, Metric{field.label, value})
	}
	inputs := map[string]*TableInput{}
	mappedRelays := map[string]bool{}
	for i := range view.Table.Inputs {
		input := view.Table.Inputs[i]
		input.Values = nil
		for _, value := range view.Table.Inputs[i].Values {
			if value.Label != "After power loss" {
				input.Values = append(input.Values, value)
			}
		}
		inputs[input.Key] = &input
		if input.RelayKey != "" {
			mappedRelays[input.RelayKey] = true
		}
	}
	for _, key := range componentKeys(config, status, "") {
		settings, _ := config[key].(map[string]any)
		reading, _ := status[key].(map[string]any)
		parts := strings.SplitN(key, ":", 2)
		kind, icon := detailKind(parts[0])
		channel := DetailChannel{Key: key, Name: kind + " " + parts[1], Kind: kind, Icon: icon, Notes: notes[key], State: "Not reported", StateTone: "muted", SavedSettings: !view.ConfigAvailable}
		if name, ok := settings["name"].(string); ok && name != "" {
			channel.Name = name
		}
		if !view.StatusAvailable {
			channel.State = "Unavailable"
		} else if output, exists := reading["output"]; exists {
			if output == nil {
				channel.State = "Not reported"
			} else {
				channel.State, channel.StateTone = boolState(output)
			}
		} else if state, exists := reading["state"]; exists {
			if value, ok := state.(bool); ok {
				channel.State, channel.StateTone = boolState(value)
			} else {
				channel.State = detailReading(state, true, "")
			}
		}
		for _, field := range []struct{ key, label, unit string }{{"current_pos", "Position", "%"}, {"brightness", "Brightness", "%"}, {"rh", "Humidity", "%"}} {
			if value, exists := reading[field.key]; exists {
				channel.Values = append(channel.Values, Metric{field.label, detailReading(value, view.StatusAvailable, field.unit)})
			}
		}
		if temp, ok := reading["temperature"].(map[string]any); ok {
			channel.Values = append(channel.Values, Metric{"Temperature", detailReading(temp["tC"], view.StatusAvailable, " °C")})
		} else if temp, exists := reading["tC"]; exists {
			channel.Values = append(channel.Values, Metric{"Temperature", detailReading(temp, view.StatusAvailable, " °C")})
		}
		for _, field := range []struct{ key, label string }{{"in_mode", "Input behavior"}, {"initial_state", "After power loss"}, {"in_locked", "Input locked"}, {"enable", "Enabled"}, {"invert", "Inverted"}, {"type", "Type"}} {
			if mappedRelays[key] && (field.key == "in_mode" || field.key == "in_locked") {
				continue
			}
			if value, exists := settings[field.key]; exists {
				text := detailReading(value, true, "")
				if setting, ok := value.(string); ok && setting != "" {
					text = inputMode(strings.ReplaceAll(setting, "_", " "))
				}
				channel.Settings = append(channel.Settings, Metric{field.label, text})
			}
		}
		if errors, ok := reading["errors"].([]any); ok && len(errors) > 0 && view.StatusAvailable {
			for _, issue := range errors {
				fault := fmt.Sprint(issue)
				view.Faults = append(view.Faults, channel.Name+": "+fault)
			}
		}
		switch parts[0] {
		case "input":
			channel.Input = inputs[key]
			if channel.Input != nil {
				channel.State, channel.StateTone = channel.Input.State, channel.Input.StateTone
				if !view.StatusAvailable {
					channel.State = "Unavailable"
				} else if channel.State == "—" {
					channel.State = "Not reported"
				}
			}
			view.Inputs = append(view.Inputs, channel)
		case "switch", "cover", "light", "rgb", "rgbw", "cct", "white":
			view.Outputs = append(view.Outputs, channel)
		default:
			view.Other = append(view.Other, channel)
		}
	}
	return view
}

func detailKind(kind string) (string, string) {
	switch kind {
	case "switch":
		return "Relay", "power"
	case "input":
		return "Input", "input"
	case "cover":
		return "Cover", "device"
	case "light", "rgb", "rgbw", "cct", "white":
		return "Light", "power"
	case "temperature":
		return "Temperature", "thermometer"
	default:
		return inputMode(kind), "device"
	}
}

func detailReading(value any, available bool, unit string) string {
	if !available {
		return "Unavailable"
	}
	if value == nil {
		return "Not reported"
	}
	switch value := value.(type) {
	case bool:
		if value {
			return "Yes"
		}
		return "No"
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64) + unit
	case string:
		if value == "" {
			return "Not reported"
		}
		return value + unit
	default:
		return inventory.Display(value) + unit
	}
}

func detailDuration(value any) string {
	seconds, ok := value.(float64)
	if !ok || seconds < 0 {
		return "Not reported"
	}
	total := int64(seconds)
	if total >= 86400 {
		return fmt.Sprintf("%dd %dh", total/86400, total%86400/3600)
	}
	if total >= 3600 {
		return fmt.Sprintf("%dh %dm", total/3600, total%3600/60)
	}
	if total >= 60 {
		return fmt.Sprintf("%dm %ds", total/60, total%60)
	}
	return fmt.Sprintf("%ds", total)
}
