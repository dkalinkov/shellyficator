package web

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"shellyficator/internal/domain"
)

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "Never"
	}
	return value.Local().Format("02 Jan, 15:04:05")
}

func display(value any) string {
	if value == nil {
		return "Not reported"
	}
	if number, ok := value.(float64); ok {
		return fmt.Sprintf("%.1f", number)
	}
	return fmt.Sprint(value)
}

func deviceView(device domain.Device) DeviceView {
	return DeviceView{
		ID: device.ID, Model: device.Model,
		Address: device.Address, Generation: device.Generation,
		Room: device.Room, Tags: device.Tags, TagsText: strings.Join(device.Tags, ", "),
		Notes: device.Notes, Problem: device.Problem, AuthEnabled: device.AuthEnabled,
		BaselineID:  device.BaselineID,
		LastChecked: formatTime(device.LastChecked), LastCaptured: formatTime(device.LastCaptured),
		HasCapture: !device.LastCaptured.IsZero(),
	}
}

func settingValue(value domain.SettingValue) string {
	if !value.Present {
		return "Not present"
	}
	encoded, _ := json.Marshal(value.Value)
	return string(encoded)
}

func organizationPage(meta PageMeta, device domain.Device, options domain.OrganizationOptions) DevicePage {
	return DevicePage{PageMeta: meta, Device: deviceView(device), Name: deviceName(device), Rooms: options.Rooms, Tags: options.Tags}
}

func differenceViews(differences []domain.Difference) []DifferenceView {
	var views []DifferenceView
	for _, difference := range differences {
		views = append(views, DifferenceView{Path: difference.Path, Before: settingValue(difference.Before), After: settingValue(difference.After)})
	}
	return views
}

func devicePage(meta PageMeta, details domain.DeviceDetails, policy domain.WatchPolicy) DevicePage {
	device := details.Device
	page := organizationPage(meta, device, details.OrganizationOptions)
	page.HasBaseline = details.Baseline != nil
	comparison := domain.CompareConfiguration(details.Baseline, device.Config, policy)
	page.Differences = differenceViews(comparison.Settings)
	page.InformationalDifferences = differenceViews(comparison.Informational)
	for _, snapshot := range details.Snapshots {
		page.Snapshots = append(page.Snapshots, SnapshotView{
			ID: snapshot.ID, Reason: snapshot.Reason, Firmware: snapshot.Firmware,
			Captured: formatTime(snapshot.Captured), CapturedISO: snapshot.Captured.Format(time.RFC3339Nano),
		})
	}
	page.Detail = detailDevice(device, details.Baseline, comparison, details.ChannelNotes)
	for _, group := range [][]DetailChannel{page.Detail.Outputs, page.Detail.Inputs, page.Detail.Other} {
		for _, channel := range group {
			page.Detail.Notes = append(page.Detail.Notes, ChannelNoteEditor{Channel: channel, DeviceID: device.ID, CSRF: page.CSRF})
		}
	}
	page.Title = page.Name
	return page
}
