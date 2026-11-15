package web

import (
	"errors"
	"net/http"

	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
)

type FirmwarePlanRow struct {
	ID, Name, Installed, Stable, Beta, DisabledReason string
	Selected                                          bool
	State                                             FirmwareView
}

type FirmwarePlanPage struct {
	PageMeta
	Rows            []FirmwarePlanRow
	Selected        int
	Single, Running bool
	URL             string
}

func (s *Server) firmwarePlan(e *core.RequestEvent) error {
	page := FirmwarePlanPage{PageMeta: s.pageMeta(e, "Update firmware"), Running: s.devices.FirmwareRunning(), URL: e.Request.URL.RequestURI()}
	var devices []Device
	if id := e.Request.PathValue("id"); id != "" {
		page.Single = true
		details, err := s.devices.Detail(id)
		if errors.Is(err, domain.ErrNotFound) {
			return e.NotFoundError("Device not found", nil)
		}
		if err != nil {
			return err
		}
		devices = []Device{{Info: deviceView(details.Device), Name: deviceName(details.Device), Table: tableDevice(details.Device, details.Baseline, 0)}}
	} else {
		inventory, err := s.list(e)
		if err != nil {
			return err
		}
		devices = inventory.Devices
	}
	for _, device := range devices {
		view := device.Table.Firmware
		if !page.Single && !view.Visible {
			continue
		}
		row := FirmwarePlanRow{ID: device.Info.ID, Name: device.Name, Installed: device.Info.Firmware, Stable: view.Stable, Beta: view.Beta, State: view}
		if row.Stable == row.Installed {
			row.Stable = ""
		}
		if row.Beta == row.Installed {
			row.Beta = ""
		}
		switch {
		case view.Active:
			row.DisabledReason = view.Message
		case page.Running:
			row.DisabledReason = "Another firmware update is running. Wait for this batch to finish."
		case !view.Current:
			row.DisabledReason = "Refresh device status to confirm it is reachable."
		case row.Installed == "":
			row.DisabledReason = "The installed firmware version has not been reported."
		case row.Stable == "" && row.Beta == "" && view.Staged:
			row.DisabledReason = "The device is completing a staged update. Wait and refresh its status."
		case row.Stable == "" && row.Beta == "" && view.Problem != "":
			row.DisabledReason = view.Problem
		case !view.Known:
			row.DisabledReason = "Update availability is unknown. Check for updates on the device page."
		case row.Stable == "" && row.Beta == "":
			row.DisabledReason = "No update is currently offered."
		}
		row.Selected = row.DisabledReason == "" && (page.Single || row.Stable != "")
		if row.Selected {
			page.Selected++
		}
		page.Rows = append(page.Rows, row)
	}
	name := "firmware-page"
	if e.Request.Header.Get("HX-Request") == "true" {
		name = "firmware-plan"
	}
	return s.render(e, name, page)
}

func (s *Server) updateFirmware(e *core.RequestEvent) error {
	if e.Request.FormValue("confirm") != "update" {
		return e.BadRequestError("Review the devices and confirm the firmware update.", nil)
	}
	var selections []domain.FirmwareSelection
	for _, id := range e.Request.Form["device"] {
		stage := e.Request.FormValue("stage_" + id)
		selections = append(selections, domain.FirmwareSelection{DeviceID: id, Stage: stage, FromVersion: e.Request.FormValue("from_" + id), TargetVersion: e.Request.FormValue("target_" + stage + "_" + id)})
	}
	err := s.devices.StartFirmwareUpdates(e.Request.Context(), selections)
	if errors.Is(err, domain.ErrFirmwareBusy) {
		return e.BadRequestError("Firmware updates are already running. Wait for this batch to finish.", nil)
	}
	if errors.Is(err, domain.ErrFirmwareSelection) || errors.Is(err, domain.ErrNotFound) {
		return e.BadRequestError("The selected devices or versions changed. Refresh and review the confirmation again.", nil)
	}
	if err != nil {
		return err
	}
	if e.Request.Header.Get("HX-Request") == "true" {
		s.notify(e, "Firmware updates queued. Progress will appear in the device status icons.", "success")
		return e.NoContent(http.StatusNoContent)
	}
	return s.redirect(e, "/", "Firmware updates queued. Progress appears in the Status column.")
}
