package web

import (
	"fmt"
	"strings"

	"shellyficator/internal/domain"
)

type FirmwareView struct {
	Installed, Stable, Beta, Label, Tone, Info string
	Checked, Problem                           string
	Known, Current, Available, Staged          bool
	ClockWarning                               bool
	Icon, Phase, Message                       string
	Active, HasResult, Visible                 bool
}

func firmwareView(device domain.Device) FirmwareView {
	view := FirmwareView{Installed: device.Firmware, Label: "Not checked", Tone: "muted", Icon: "update"}
	sys, _ := device.Status["sys"].(map[string]any)
	available, known := sys["available_updates"].(map[string]any)
	readAt := device.LastStatusRead
	view.Current = device.StatusCurrent()
	if check := device.FirmwareCheck; check != nil && check.Firmware == device.Firmware {
		view.Checked, view.Problem = formatTime(check.CheckedAt), check.Problem
		if check.Problem == "" && (!known || !check.CheckedAt.Before(readAt)) {
			available, known, readAt = check.Available, check.Available != nil, check.CheckedAt
		}
	}
	if view.Installed == "" {
		view.Installed = "Not reported"
	}
	view.Known = known
	for _, stage := range []string{"stable", "beta"} {
		release, _ := available[stage].(map[string]any)
		version, _ := release["version"].(string)
		version = strings.TrimSpace(version)
		// A same-version offer can be a remaining pass of a staged update.
		_, staged := release["nsteps"].(float64)
		// The explicit check response may omit nsteps; retain it from system
		// status only when both sources describe the same offered release.
		cached, _ := sys["available_updates"].(map[string]any)
		cachedRelease, _ := cached[stage].(map[string]any)
		if cachedRelease["version"] == version {
			_, cachedStaged := cachedRelease["nsteps"].(float64)
			staged = staged || cachedStaged
		}
		if version == "" || version == device.Firmware && !staged {
			continue
		}
		if stage == "stable" {
			view.Stable = version
		} else {
			view.Beta = version
		}
		view.Staged = view.Staged || staged
	}
	view.Available = view.Stable != "" || view.Beta != ""
	switch {
	case view.Stable != "":
		view.Label, view.Tone = "Update available", "warning"
		view.Info = "Stable firmware " + view.Stable + " available"
	case view.Beta != "":
		view.Label, view.Tone = "Beta available", "blue"
		view.Info = "Beta firmware " + view.Beta + " available"
	case known:
		view.Label = "No update reported"
		view.Info = "Device reports no stable or beta update"
	default:
		view.Info = "Update availability has not been reported. Check for updates on the device page."
	}
	if view.Staged {
		view.Info += "\nDevice reports a staged update; the final pass may finish automatically after reboot."
	}
	if known && !view.Current {
		view.Label, view.Tone = "Last reported · "+view.Label, "muted"
		view.Info += "\nLast reported; device status is unavailable."
	}
	if known {
		stamp := "Unknown"
		if !readAt.IsZero() {
			stamp = formatTime(readAt)
		}
		view.Info += "\nAvailability read · " + stamp
	}
	if view.Problem != "" {
		view.Label, view.Tone = "Check failed", "error"
		view.Icon = "warning"
		view.Info += "\n" + view.Problem
	}
	view.Info += fmt.Sprintf("\nInstalled firmware %s", view.Installed)
	if unix, present := sys["unixtime"]; present && device.StatusCurrent() {
		seconds, synced := unix.(float64)
		view.ClockWarning = !synced || seconds <= 0
	}
	if update := device.FirmwareUpdate; update != nil && (update.Phase != "succeeded" || device.Firmware == update.TargetVersion) {
		view.Active, view.HasResult = update.Active(), true
		view.Phase, view.Message = update.Phase, update.Message
		if update.Phase == "succeeded" && view.Available {
			// A completed attempt must not hide a newly offered stable or beta release.
			view.Info += "\nLast update: " + update.Message
			view.Visible = true
			return view
		}
		switch update.Phase {
		case "queued":
			view.Label, view.Tone, view.Icon = "Update queued", "blue", "clock"
		case "capturing":
			view.Label, view.Tone = "Capturing settings", "blue"
		case "starting":
			view.Label, view.Tone = "Starting update", "blue"
		case "verifying":
			view.Label, view.Tone = "Updating · verifying", "blue"
		case "succeeded":
			view.Label, view.Tone, view.Icon = "Updated", "ok", "check-circle"
		case "failed":
			view.Label, view.Tone, view.Icon = "Update failed", "error", "warning"
		case "unconfirmed":
			view.Label, view.Tone, view.Icon = "Update not verified", "warning", "warning"
		case "warning":
			view.Label, view.Tone, view.Icon = "Updated · capture unavailable", "warning", "warning"
		}
		view.Info = view.Label + "\n" + update.Message + "\n" + update.FromVersion + " → " + update.TargetVersion + " (" + update.Stage + ")\nLast progress · " + formatTime(update.UpdatedAt) + "\nInstalled firmware " + view.Installed
	}
	view.Visible = view.Available || view.Problem != "" || view.HasResult
	return view
}
