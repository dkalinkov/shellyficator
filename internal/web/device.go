package web

import (
	"context"
	"errors"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/inventory"
	"shellyficator/internal/shelly"
)

func (s *Server) detail(e *core.RequestEvent) error {
	record, err := s.store.Device(e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Device not found", nil)
	}
	page := DevicePage{PageMeta: s.pageMeta(e, "Device"), Device: record, Name: deviceName(record)}
	page.Rooms, page.Tags, err = s.store.OrganizationOptions()
	if err != nil {
		return err
	}
	config := inventory.Object(record, "config")
	baseline := s.store.Baseline(record)
	page.HasBaseline = baseline != nil
	if page.HasBaseline && config != nil {
		page.Differences = inventory.Differences(baseline, config)
	}
	page.Snapshots, err = s.store.Snapshots(record.Id)
	if err != nil {
		return err
	}
	page.Detail = detailDevice(record, baseline, page.Differences, s.store.ChannelNotes(record.Id))
	for _, group := range [][]DetailChannel{page.Detail.Outputs, page.Detail.Inputs, page.Detail.Other} {
		for _, channel := range group {
			page.Detail.Notes = append(page.Detail.Notes, ChannelNoteEditor{Channel: channel, DeviceID: record.Id, CSRF: page.CSRF})
		}
	}
	page.Title = page.Name
	return s.render(e, "detail", page)
}

func (s *Server) refreshDevice(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	record, err := s.store.Device(id)
	if err != nil {
		return e.NotFoundError("Device not found", nil)
	}
	ctx, cancel := context.WithTimeout(e.Request.Context(), 12*time.Second)
	defer cancel()
	reading, err := s.client.Read(ctx, record.GetString("address"), record.GetString("mac"), false)
	if err != nil {
		if saveErr := s.store.Failed(id, "Status could not be read. Check device identity and connectivity.", errors.Is(err, shelly.ErrIdentity)); saveErr != nil {
			return saveErr
		}
		return s.redirect(e, "/devices/"+id, "Refresh failed. Check device access.")
	}
	// A status refresh must retain a warning about an unavailable configuration.
	if !record.GetBool("config_available") && reading.Problem == "" {
		reading.Problem = "Configuration is unavailable. Capture config to retry."
	}
	if _, err := s.store.SaveReading(reading, false); err != nil {
		return err
	}
	if reading.Status == nil {
		return s.redirect(e, "/devices/"+id, "Device reached, but status could not be read.")
	}
	return s.redirect(e, "/devices/"+id, "Status refreshed.")
}

func (s *Server) metadata(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if err := s.store.Metadata(id, e.Request.FormValue("room"), e.Request.FormValue("tags"), e.Request.FormValue("notes")); err != nil {
		return e.BadRequestError("Could not save device information", nil)
	}
	return s.redirect(e, "/devices/"+id, "Device info saved.")
}

func (s *Server) organizationEditor(e *core.RequestEvent) error {
	record, err := s.store.Device(e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Device not found", nil)
	}
	page := DevicePage{PageMeta: s.pageMeta(e, "Device"), Device: record, Name: deviceName(record)}
	page.Rooms, page.Tags, err = s.store.OrganizationOptions()
	if err != nil {
		return err
	}
	return s.render(e, "organization-editor", page)
}

func (s *Server) organization(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if err := s.store.Organization(id, e.Request.FormValue("room"), e.Request.FormValue("tags")); err != nil {
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
	return s.render(e, "organization-saved", page)
}

func (s *Server) channelNote(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	if err := s.store.SaveChannelNote(id, e.Request.FormValue("component"), e.Request.FormValue("notes")); err != nil {
		return e.BadRequestError("Could not save channel note", nil)
	}
	return s.redirect(e, "/devices/"+id, "Note saved.")
}

func (s *Server) readDeviceConfig(e *core.RequestEvent) (shelly.Reading, error) {
	id := e.Request.PathValue("id")
	record, err := s.store.Device(id)
	if err != nil {
		return shelly.Reading{}, err
	}
	ctx, cancel := context.WithTimeout(e.Request.Context(), 20*time.Second)
	defer cancel()
	reading, err := s.client.Read(ctx, record.GetString("address"), record.GetString("mac"), true)
	if err != nil {
		if saveErr := s.store.Failed(id, "Configuration could not be captured. Check device identity and connectivity.", errors.Is(err, shelly.ErrIdentity)); saveErr != nil {
			return reading, saveErr
		}
		return reading, err
	}
	if reading.Config == nil {
		if _, err := s.store.SaveReading(reading, false); err != nil {
			return reading, err
		}
		return reading, errors.New("configuration could not be read; check connectivity and shared device credentials")
	}
	return reading, nil
}

func (s *Server) capture(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	reading, err := s.readDeviceConfig(e)
	if err == nil {
		_, err = s.store.SaveReading(reading, true)
	}
	if err != nil {
		return s.redirect(e, "/devices/"+id, "Capture failed. Check device access.")
	}
	return s.redirect(e, "/devices/"+id, "Configuration captured.")
}

func (s *Server) baseline(e *core.RequestEvent) error {
	id := e.Request.PathValue("id")
	reading, err := s.readDeviceConfig(e)
	if err != nil {
		return s.redirect(e, "/devices/"+id, "Baseline unchanged: config read failed.")
	}
	if _, err := s.store.ApproveReading(reading); err != nil {
		return e.BadRequestError("Could not approve baseline", nil)
	}
	return s.redirect(e, "/devices/"+id, "Baseline updated.")
}
