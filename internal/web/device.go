package web

import (
	"errors"

	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/domain"
)

func (s *Server) detail(e *core.RequestEvent) error {
	details, err := s.devices.Detail(e.Request.PathValue("id"))
	if errors.Is(err, domain.ErrNotFound) {
		return e.NotFoundError("Device not found", nil)
	}
	if err != nil {
		return err
	}
	meta := s.pageMeta(e, "Device")
	if e.Request.Header.Get("HX-Request") == "true" {
		s.notify(e, meta.Notice, meta.NoticeLevel)
	}
	return s.render(e, "detail", devicePage(meta, details))
}

func (s *Server) refreshDevice(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	result, err := s.devices.RefreshDevice(e.Request.Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		return e.NotFoundError("Device not found", nil)
	}
	if errors.Is(err, domain.ErrDeviceRead) {
		return s.redirect(e, "/devices/"+id, "Refresh failed. Check device access.", "error")
	}
	if err != nil {
		return err
	}
	if !result.StatusAvailable {
		return s.redirect(e, "/devices/"+id, "Device reached, but status could not be read.", "warning")
	}
	return s.redirect(e, "/devices/"+id, "Status refreshed.")
}

func (s *Server) checkFirmware(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	device, err := s.devices.CheckFirmware(e.Request.Context(), id)
	if errors.Is(err, domain.ErrNotFound) {
		return e.NotFoundError("Device not found", nil)
	}
	if errors.Is(err, domain.ErrDeviceRead) {
		return s.redirect(e, "/devices/"+id, "Firmware check failed. Check device access.", "error")
	}
	if err != nil {
		return err
	}
	if device.FirmwareCheck != nil && device.FirmwareCheck.Problem != "" {
		return s.redirect(e, "/devices/"+id, device.FirmwareCheck.Problem, "warning")
	}
	return s.redirect(e, "/devices/"+id, "Firmware availability checked.")
}

func (s *Server) metadata(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if err := s.devices.Metadata(id, e.Request.FormValue("room"), e.Request.FormValue("tags"), e.Request.FormValue("notes")); err != nil {
		return e.BadRequestError("Could not save device information", nil)
	}
	return s.redirect(e, "/devices/"+id, "Device info saved.")
}

func (s *Server) organizationEditor(e *core.RequestEvent) error {
	details, err := s.devices.Detail(e.Request.PathValue("id"))
	if errors.Is(err, domain.ErrNotFound) {
		return e.NotFoundError("Device not found", nil)
	}
	if err != nil {
		return err
	}
	return s.render(e, "organization-editor", devicePage(s.pageMeta(e, "Device"), details))
}

func (s *Server) organization(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if err := s.devices.Organization(id, e.Request.FormValue("room"), e.Request.FormValue("tags")); err != nil {
		return e.BadRequestError("Could not save room & tags. Room names allow 100 characters; tags allow 500 in total.", nil)
	}
	if e.Request.Header.Get("HX-Request") != "true" {
		return s.redirect(e, "/", "Room & tags saved.")
	}
	page, err := s.listFiltered(e, e.Request.FormValue("filter_q"), e.Request.FormValue("filter_room"), e.Request.FormValue("filter_tags"), e.Request.FormValue("filter_group"))
	if err != nil {
		return err
	}
	e.Response.Header().Set("HX-Replace-Url", inventoryURL(page))
	s.notify(e, "Room & tags saved.", "success")
	return s.render(e, "organization-saved", page)
}

func (s *Server) channelNote(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if err := s.devices.SaveChannelNote(id, e.Request.FormValue("component"), e.Request.FormValue("notes")); err != nil {
		return e.BadRequestError("Could not save channel note", nil)
	}
	return s.redirect(e, "/devices/"+id, "Note saved.")
}

func (s *Server) capture(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	_, err := s.devices.Capture(e.Request.Context(), id)
	if err != nil {
		return s.redirect(e, "/devices/"+id, "Capture failed. Check device access.", "error")
	}
	return s.redirect(e, "/devices/"+id, "Configuration captured.")
}

func (s *Server) baseline(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	_, err := s.devices.ApproveBaseline(e.Request.Context(), id)
	if errors.Is(err, domain.ErrBaselineSave) {
		return e.BadRequestError("Could not approve baseline", nil)
	}
	if err != nil {
		return s.redirect(e, "/devices/"+id, "Baseline unchanged: config read failed.", "error")
	}
	return s.redirect(e, "/devices/"+id, "Baseline updated.")
}
