package web

type ConnectionState struct {
	Value, Tone, Info string
}

type ConnectionField struct {
	Label, Value, Tone, Icon, Info string
	State                          bool
}

// A reported connection takes precedence over saved configuration. Unavailable
// reads must never turn a cached connection or enable flag into a current state.
func serviceConnection(status, config map[string]any, statusAvailable, configAvailable bool) ConnectionState {
	if !statusAvailable {
		return ConnectionState{"Unavailable", "muted", "Connection could not be checked."}
	}
	connected, reported := status["connected"].(bool)
	if reported && connected {
		return ConnectionState{"Connected", "ok", "The device reports an active connection."}
	}
	enabled, configured := config["enable"].(bool)
	if configAvailable && configured && !enabled {
		return ConnectionState{"Disabled", "muted", "Disabled in the device settings."}
	}
	if reported {
		return ConnectionState{"Disconnected", "warning", "The device reports no connection."}
	}
	return ConnectionState{"Unknown", "muted", "The device did not report a connection state."}
}

func wifiConnection(value any, available bool) ConnectionState {
	if !available {
		return ConnectionState{Value: "Unavailable", Tone: "muted"}
	}
	switch value {
	case "connected", "got ip":
		return ConnectionState{Value: "Connected", Tone: "ok"}
	case "connecting":
		return ConnectionState{Value: "Connecting", Tone: "blue"}
	case "disconnected":
		return ConnectionState{Value: "Disconnected", Tone: "error"}
	default:
		return ConnectionState{Value: detailReading(value, true, ""), Tone: "muted"}
	}
}
