package web

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"shelly-manager/internal/inventory"
)

type DeviceTable struct {
	Model, Identity, Address, Note, Signal              string
	SignalValue, SignalTone                             string
	Connectivity, ConnectivityTone, ConnectionInfo      string
	Configuration, ConfigurationTone, ConfigurationInfo string
	Inputs                                              []TableInput
	CachedInputs, InputsChanged                         bool
}

type TableInput struct {
	RelayID                                                string
	ID, Key, Name, Type, Mode, ModeTone, Info, Restriction string
	RelayKey, Note, RelayNote                              string
	State, StateTone, Output, OutputTone                   string
	Values                                                 []Metric
	Changed                                                bool
}

func tableDevice(record *core.Record, baseline map[string]any, differences int) DeviceTable {
	status, config := inventory.Object(record, "status"), inventory.Object(record, "config")
	model := record.GetString("model")
	if name := map[string]string{
		"S4SW-001P8EU": "Mini 1PM", "S4SW-002P16EU": "2PM",
		"S3SW-002P16EU": "2PM", "SNSW-001P16EU": "Plus 1PM",
		"S3PB-O3AR000001": "Ogemray 25A",
	}[model]; name != "" {
		model = name
	}
	view := DeviceTable{
		Model: model, Connectivity: "Reachable", ConnectivityTone: "ok",
		SignalValue: "—", SignalTone: "muted",
		Configuration: "No baseline", ConfigurationTone: "muted",
		Identity:          fmt.Sprintf("%s\n%s · Gen %d\nFirmware %s\nMAC %s", record.GetString("device_id"), record.GetString("model"), record.GetInt("generation"), record.GetString("firmware"), record.GetString("mac")),
		ConnectionInfo:    "Checked " + inventory.FormatTime(record.GetString("last_checked")) + "\nLast seen " + inventory.FormatTime(record.GetString("last_seen")),
		ConfigurationInfo: "Captured " + inventory.FormatTime(record.GetString("last_captured")),
		Note:              previewText(record.GetString("notes"), 320),
		CachedInputs:      !record.GetBool("reachable") || !record.GetBool("config_available"),
	}
	if room := record.GetString("room"); room != "" {
		view.Identity += "\nRoom: " + room
	}
	if tags := record.GetString("tags"); tags != "" {
		view.Identity += "\nTags: " + tags
	}
	if parsed, err := url.Parse(record.GetString("address")); err == nil {
		view.Address = parsed.Host
	}
	if !record.GetBool("reachable") {
		view.Connectivity, view.ConnectivityTone = "Offline", "error"
	} else if !record.GetBool("status_available") {
		view.Connectivity, view.ConnectivityTone = "Read failed", "warning"
	}
	if problem := record.GetString("problem"); problem != "" {
		view.ConnectionInfo += "\n" + problem
	}
	currentStatus := record.GetBool("reachable") && record.GetBool("status_available")
	if wifi, ok := status["wifi"].(map[string]any); ok {
		if rssi, ok := wifi["rssi"].(float64); ok {
			view.SignalValue = fmt.Sprintf("%.0f", rssi)
			view.Signal = fmt.Sprintf("Wi-Fi %.0f dBm", rssi)
			if currentStatus {
				switch {
				case rssi >= -60:
					view.SignalTone = "ok"
				case rssi >= -70:
					view.SignalTone = "warning"
				default:
					view.SignalTone = "error"
				}
			} else {
				view.Signal += " · last reported"
			}
		}
	}
	if baseline != nil {
		view.Configuration, view.ConfigurationTone = "Unavailable", "warning"
		if record.GetBool("reachable") && record.GetBool("config_available") {
			view.Configuration, view.ConfigurationTone = "Matches", "ok"
			if differences > 0 {
				view.Configuration, view.ConfigurationTone = fmt.Sprintf("%d changes", differences), "warning"
				if differences == 1 {
					view.Configuration = "1 change"
				}
			}
		}
		view.ConfigurationInfo += "\nCompared with approved baseline"
	} else {
		view.ConfigurationInfo += "\nBaseline not approved"
	}

	switches := componentKeys(config, status, "switch:")
	for _, key := range componentKeys(config, status, "input:") {
		input, _ := config[key].(map[string]any)
		inputReading, _ := status[key].(map[string]any)
		id := strings.TrimPrefix(key, "input:")
		line := TableInput{ID: id, Key: key, Type: inputType(input["type"]), Mode: "—", ModeTone: "muted", State: "—", StateTone: "muted", Output: "—", OutputTone: "muted"}
		line.Name, _ = input["name"].(string)
		var settings, reading map[string]any
		for _, switchKey := range switches {
			switchConfig, _ := config[switchKey].(map[string]any)
			inputID := strings.TrimPrefix(switchKey, "switch:")
			if mapped, ok := switchConfig["input_id"].(float64); ok {
				inputID = fmt.Sprintf("%.0f", mapped)
			}
			if inputID == id {
				line.RelayKey, settings = switchKey, switchConfig
				line.RelayID = strings.TrimPrefix(switchKey, "switch:")
				reading, _ = status[switchKey].(map[string]any)
				line.Mode = inputMode(settings["in_mode"])
				line.ModeTone = modeTone(settings["in_mode"])
				break
			}
		}
		line.Info = fmt.Sprintf("%s: %s\nMode: %s\nCaptured %s", key, line.Type, line.Mode, inventory.FormatTime(record.GetString("last_captured")))
		if currentStatus {
			if state, exists := inputReading["state"]; exists {
				line.State, line.StateTone = boolState(state)
				if state == nil {
					line.State = "Not reported"
				}
			}
			line.Output, line.OutputTone = boolState(reading["output"])
		}
		if locked, _ := settings["in_locked"].(bool); locked {
			line.Restriction = "Locked"
		}
		if enabled, exists := input["enable"].(bool); exists && !enabled {
			line.Restriction = strings.TrimSpace(line.Restriction + " Disabled")
		}
		for _, field := range []struct {
			label string
			value any
		}{
			{"Enabled", input["enable"]}, {"Locked", settings["in_locked"]}, {"Inverted", input["invert"]},
		} {
			if value, ok := field.value.(bool); ok {
				text := "No"
				if value {
					text = "Yes"
				}
				line.Values = append(line.Values, Metric{field.label, text})
			}
		}
		if value, ok := settings["initial_state"].(string); ok && value != "" {
			line.Values = append(line.Values, Metric{"After power loss", inputMode(strings.ReplaceAll(value, "_", " "))})
		}
		if baseline != nil && !view.CachedInputs {
			beforeSwitch, _ := baseline[line.RelayKey].(map[string]any)
			beforeInput, _ := baseline[key].(map[string]any)
			for _, pair := range []struct {
				label         string
				before, after any
			}{
				{"Input", beforeSwitch["input_id"], settings["input_id"]},
				{"Mode", beforeSwitch["in_mode"], settings["in_mode"]},
				{"Input type", beforeInput["type"], input["type"]},
				{"Locked", beforeSwitch["in_locked"], settings["in_locked"]},
				{"Enabled", beforeInput["enable"], input["enable"]},
				{"Inverted", beforeInput["invert"], input["invert"]},
			} {
				if fmt.Sprint(pair.before) != fmt.Sprint(pair.after) {
					line.Changed = true
					line.Info += fmt.Sprintf("\n%s: %s → %s", pair.label, inventory.Display(pair.before), inventory.Display(pair.after))
				}
			}
		}
		view.InputsChanged = view.InputsChanged || line.Changed
		view.Inputs = append(view.Inputs, line)
	}
	if currentStatus {
		var faults []string
		for key, value := range status {
			component, _ := value.(map[string]any)
			if reported, ok := component["errors"].([]any); ok {
				for _, issue := range reported {
					faults = append(faults, fmt.Sprintf("%s: %v", key, issue))
				}
			}
		}
		if len(faults) > 0 {
			view.Connectivity, view.ConnectivityTone = "Device fault", "error"
			sort.Strings(faults)
			view.ConnectionInfo += "\n" + strings.Join(faults, "\n")
		}
	}
	return view
}

func componentKeys(config, status map[string]any, prefix string) []string {
	keys := map[string]bool{}
	for _, source := range []map[string]any{config, status} {
		for key := range source {
			if strings.HasPrefix(key, prefix) {
				keys[key] = true
			}
		}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool {
		a, _ := strconv.Atoi(strings.TrimPrefix(ordered[i], prefix))
		b, _ := strconv.Atoi(strings.TrimPrefix(ordered[j], prefix))
		return a < b
	})
	return ordered
}

func boolState(value any) (string, string) {
	if state, ok := value.(bool); ok {
		if state {
			return "On", "ok"
		}
		return "Off", "muted"
	}
	return "—", "muted"
}

func modeTone(value any) string {
	switch value {
	case "momentary", "activate":
		return "blue"
	case "follow":
		return "ok"
	case "detached":
		return "violet"
	case "flip", "cycle":
		return "pink"
	default:
		return "muted"
	}
}

func inputType(value any) string {
	switch value {
	case "button":
		return "Button"
	case "switch":
		return "Toggle"
	case "analog":
		return "Analog"
	case "count":
		return "Counter"
	default:
		return "—"
	}
}

func inputMode(value any) string {
	mode, ok := value.(string)
	if !ok || mode == "" {
		return "—"
	}
	return strings.ToUpper(mode[:1]) + mode[1:]
}

func previewText(value string, limit int) string {
	characters := []rune(strings.TrimSpace(value))
	if len(characters) > limit {
		return string(characters[:limit]) + "…"
	}
	return string(characters)
}
