package devices

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"shellyficator/internal/domain"
)

const firmwareSettle = 2 * time.Minute

func (s *Service) recoverFirmwareUpdates() {
	if s.store == nil {
		return
	}
	devices, err := s.store.Devices()
	if err != nil {
		s.logger.Error("Could not recover firmware update status", "error", err)
		return
	}
	updates := map[string]domain.FirmwareUpdate{}
	for _, device := range devices {
		if !device.FirmwareUpdate.Active() {
			continue
		}
		u := *device.FirmwareUpdate
		if u.Phase == "queued" || u.Phase == "capturing" {
			u.Phase, u.Message = "failed", "Not started: the app stopped before the update command was sent."
		} else {
			u.Phase, u.Message = "unconfirmed", "The app restarted before verification finished. Check the installed version before retrying."
		}
		u.UpdatedAt = time.Now().UTC()
		updates[device.ID] = u
	}
	if len(updates) > 0 {
		if err := s.store.SaveFirmwareUpdates(updates); err != nil {
			s.pendingFirmwareResults = updates
			s.logger.Error("Could not recover interrupted firmware updates", "error", err)
		}
	}
}

// StartFirmwareUpdates admits a confirmed batch. Work belongs to the service,
// so closing a browser tab cannot cancel an already accepted update command.
func (s *Service) StartFirmwareUpdates(caller context.Context, selections []domain.FirmwareSelection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return domain.ErrClosed
	}
	if s.firmwareRunning {
		return domain.ErrFirmwareBusy
	}
	if len(selections) == 0 || len(selections) > 64 {
		return domain.ErrFirmwareSelection
	}
	if err := caller.Err(); err != nil {
		return err
	}
	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	s.retryFirmwareResultsLocked()
	updates := map[string]domain.FirmwareUpdate{}
	queue := slices.Clone(selections)
	for _, selection := range queue {
		if _, duplicate := updates[selection.DeviceID]; duplicate || selection.FromVersion == "" || selection.TargetVersion == "" || selection.FromVersion == selection.TargetVersion || len(selection.TargetVersion) > 100 || len(selection.FromVersion) > 100 || selection.Stage != "stable" && selection.Stage != "beta" {
			return domain.ErrFirmwareSelection
		}
		device, err := s.store.Device(selection.DeviceID)
		if err != nil {
			return err
		}
		if device.FirmwareUpdate.Active() {
			return domain.ErrFirmwareBusy
		}
		if device.Firmware != selection.FromVersion || !device.StatusCurrent() {
			return fmt.Errorf("%w: refresh %s before updating", domain.ErrFirmwareSelection, device.DeviceID)
		}
		now := time.Now().UTC()
		updates[device.ID] = domain.FirmwareUpdate{ID: rand.Text(), Stage: selection.Stage, FromVersion: selection.FromVersion, TargetVersion: selection.TargetVersion, Phase: "queued", Message: "Queued; updates run one device at a time.", UpdatedAt: now}
	}
	if err := caller.Err(); err != nil {
		return err
	}
	if err := s.store.SaveFirmwareUpdates(updates); err != nil {
		return err
	}
	s.firmwareRunning = true
	s.jobs.Go(func() {
		defer func() { s.mu.Lock(); s.firmwareRunning = false; s.mu.Unlock() }()
		for _, selection := range queue {
			u := updates[selection.DeviceID]
			if s.ctx.Err() != nil {
				s.finishFirmware(selection.DeviceID, &u, "failed", "Not started: the app stopped before this device's turn.")
				continue
			}
			s.runFirmwareUpdate(selection, u)
		}
	})
	return nil
}

func (s *Service) FirmwareRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.firmwareRunning
}

func (s *Service) publishFirmware(id string, u *domain.FirmwareUpdate, phase, message string) error {
	u.Phase, u.Message, u.UpdatedAt = phase, message, time.Now().UTC()
	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	err := s.store.SaveFirmwareUpdates(map[string]domain.FirmwareUpdate{id: *u})
	if err != nil {
		s.logger.Error("Could not save firmware update progress", "device", id, "error", err)
		if !u.Active() && !errors.Is(err, domain.ErrNotFound) {
			if s.pendingFirmwareResults == nil {
				s.pendingFirmwareResults = make(map[string]domain.FirmwareUpdate)
			}
			s.pendingFirmwareResults[id] = *u
		}
	}
	return err
}

func (s *Service) finishFirmware(id string, u *domain.FirmwareUpdate, phase, message string) {
	_ = s.publishFirmware(id, u, phase, message)
}

// Retry result writes on subsequent requests. The device command runs only in its original job.
func (s *Service) retryFirmwareResults() {
	s.inventoryMu.RLock()
	pending := len(s.pendingFirmwareResults) > 0
	s.inventoryMu.RUnlock()
	if pending {
		s.inventoryMu.Lock()
		defer s.inventoryMu.Unlock()
		s.retryFirmwareResultsLocked()
	}
}

func (s *Service) retryFirmwareResultsLocked() {
	for id, result := range s.pendingFirmwareResults {
		device, err := s.store.Device(id)
		if err == nil && device.FirmwareUpdate.Active() && device.FirmwareUpdate.ID == result.ID {
			err = s.store.SaveFirmwareUpdates(map[string]domain.FirmwareUpdate{id: result})
		}
		if err == nil || errors.Is(err, domain.ErrNotFound) {
			delete(s.pendingFirmwareResults, id)
		}
	}
}

func (s *Service) saveFirmwareReading(device domain.Device, reading domain.Reading, capture bool) error {
	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	latest, err := s.store.Device(device.ID)
	if err != nil {
		return err
	}
	if deviceMoved(device, latest) {
		return domain.ErrDeviceMoved
	}
	reading.Address = device.Address
	_, err = s.store.SaveReading(reading, capture)
	return err
}

func (s *Service) runFirmwareUpdate(selection domain.FirmwareSelection, u domain.FirmwareUpdate) {
	id := selection.DeviceID
	ctx, cancel := context.WithTimeout(s.ctx, 12*time.Minute)
	defer cancel()
	if err := s.publishFirmware(id, &u, "capturing", "Checking the offered release and capturing current settings."); err != nil {
		s.finishFirmware(id, &u, "failed", "Could not save update progress. No update command was sent.")
		return
	}
	device, err := s.savedDevice(id)
	if err != nil {
		s.finishFirmware(id, &u, "failed", "Could not load the saved device. No update command was sent.")
		return
	}
	checkCtx, stopCheck := context.WithTimeout(ctx, 40*time.Second)
	checked, err := s.client.CheckForUpdate(checkCtx, device.Address, device.MAC)
	stopCheck()
	if err == nil {
		if saveErr := s.saveFirmwareReading(device, checked, false); saveErr != nil {
			s.finishFirmware(id, &u, "failed", "Could not save the fresh firmware check. No update command was sent.")
			return
		}
	}
	if err != nil || checked.FirmwareCheck == nil || checked.FirmwareCheck.Problem != "" {
		message := "Could not confirm the offered firmware. No update command was sent."
		if checked.FirmwareCheck != nil && checked.FirmwareCheck.Problem != "" {
			message = checked.FirmwareCheck.Problem + " No update command was sent."
		}
		s.finishFirmware(id, &u, "failed", message)
		return
	}
	release, _ := checked.FirmwareCheck.Available[selection.Stage].(map[string]any)
	version, _ := release["version"].(string)
	if strings.TrimSpace(version) != selection.TargetVersion || checked.Identity.Version != selection.FromVersion {
		s.finishFirmware(id, &u, "failed", "The installed or offered version changed. Review the versions and confirm again. No update command was sent.")
		return
	}
	readCtx, stopRead := context.WithTimeout(ctx, 20*time.Second)
	before, err := s.client.Read(readCtx, device.Address, device.MAC, true)
	stopRead()
	if err != nil || before.Config == nil || before.Status == nil || before.Identity.Version != selection.FromVersion {
		s.finishFirmware(id, &u, "failed", "Current settings and status could not be captured. No update command was sent.")
		return
	}
	before.FirmwareCheck = checked.FirmwareCheck
	before.CaptureReason = "Before firmware update to " + selection.TargetVersion
	if err := s.saveFirmwareReading(device, before, true); err != nil {
		s.finishFirmware(id, &u, "failed", "Could not save the configuration capture. No update command was sent.")
		return
	}
	if ctx.Err() != nil {
		s.finishFirmware(id, &u, "failed", "Not started: the app stopped before the update command was sent.")
		return
	}
	if err := s.publishFirmware(id, &u, "starting", "Settings captured. Sending the firmware update request."); err != nil {
		s.finishFirmware(id, &u, "failed", "Could not save update progress. No update command was sent.")
		return
	}
	writeCtx, stopWrite := context.WithTimeout(ctx, 40*time.Second)
	err = s.client.StartFirmwareUpdate(writeCtx, device.Address, device.MAC, selection.Stage)
	stopWrite()
	if errors.Is(err, domain.ErrFirmwareRejected) || errors.Is(err, domain.ErrFirmwareSelection) {
		s.finishFirmware(id, &u, "failed", "Update request rejected: "+err.Error())
		return
	}
	message := "Waiting for the device to restart, then verifying the installed firmware."
	if err != nil {
		message = "The request response was lost. Checking the installed version; the update command will not be repeated."
	}
	_ = s.publishFirmware(id, &u, "verifying", message)
	s.verifyFirmware(ctx, device, before, &u)
}

func (s *Service) verifyFirmware(ctx context.Context, device domain.Device, before domain.Reading, u *domain.FirmwareUpdate) {
	var targetSince, oldAfterRestart time.Time
	previousUptime, haveUptime := readingUptime(before)
	restarted := false
	lastVersion := u.FromVersion
	for {
		if ctx.Err() != nil {
			message := "Could not verify the installed version before the deadline. Last reported version: " + lastVersion + ". Check the device before retrying."
			if s.ctx.Err() != nil {
				message = "The app stopped before verification finished. Check the installed version before retrying."
			}
			s.finishFirmware(device.ID, u, "unconfirmed", message)
			return
		}
		readCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		reading, err := s.client.Read(readCtx, device.Address, device.MAC, false)
		cancel()
		if errors.Is(err, domain.ErrIdentity) {
			s.finishFirmware(device.ID, u, "failed", "The device identity at this address changed. Verification stopped.")
			return
		}
		if err == nil {
			lastVersion = reading.Identity.Version
			if uptime, ok := readingUptime(reading); ok {
				if haveUptime && uptime < previousUptime {
					restarted = true
					targetSince = time.Time{}
				}
				previousUptime, haveUptime = uptime, true
			}
			if err := s.saveFirmwareReading(device, reading, false); errors.Is(err, domain.ErrDeviceMoved) {
				s.finishFirmware(device.ID, u, "unconfirmed", "The device address changed during verification. Check the installed version before retrying.")
				return
			}
			now := time.Now()
			if firmwareStepPending(reading, u.Stage, u.TargetVersion) {
				targetSince, oldAfterRestart = time.Time{}, time.Time{}
				message := "The device reports a staged update. Waiting for the remaining step before verifying the version."
				if u.Message != message {
					_ = s.publishFirmware(device.ID, u, "verifying", message)
				}
			} else if lastVersion == u.TargetVersion {
				oldAfterRestart = time.Time{}
				if targetSince.IsZero() {
					targetSince = now
					_ = s.publishFirmware(device.ID, u, "verifying", "Running "+lastVersion+". Waiting for the firmware to settle before confirming success.")
				}
				if now.Sub(targetSince) >= firmwareSettle {
					s.captureAfterFirmware(ctx, device, u)
					return
				}
			} else {
				targetSince = time.Time{}
				if restarted {
					if oldAfterRestart.IsZero() {
						oldAfterRestart = now
						_ = s.publishFirmware(device.ID, u, "verifying", "Device restarted on "+lastVersion+". Waiting to check for a remaining update step.")
					}
					if now.Sub(oldAfterRestart) >= firmwareSettle {
						s.finishFirmware(device.ID, u, "failed", "Version unchanged or unexpected after restart: running "+lastVersion+", expected "+u.TargetVersion+". The update did not take effect.")
						return
					}
				}
			}
		} else {
			targetSince = time.Time{}
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func (s *Service) captureAfterFirmware(ctx context.Context, device domain.Device, u *domain.FirmwareUpdate) {
	readCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	after, err := s.client.Read(readCtx, device.Address, device.MAC, true)
	if err != nil || after.Identity.Version != u.TargetVersion {
		s.finishFirmware(device.ID, u, "unconfirmed", "The final version check failed. Check the installed firmware before retrying.")
		return
	}
	after.CaptureReason = "After firmware update to " + u.TargetVersion
	saveErr := s.saveFirmwareReading(device, after, after.Config != nil)
	if errors.Is(saveErr, domain.ErrDeviceMoved) {
		s.finishFirmware(device.ID, u, "unconfirmed", "The device address changed during verification. Check the installed version before retrying.")
		return
	}
	if after.Config == nil {
		s.finishFirmware(device.ID, u, "warning", "Updated to "+u.TargetVersion+", but settings could not be captured afterward. Capture config to retry.")
		return
	}
	if saveErr != nil {
		s.finishFirmware(device.ID, u, "warning", "Updated to "+u.TargetVersion+", but the configuration capture could not be saved.")
		return
	}
	s.finishFirmware(device.ID, u, "succeeded", "Updated to "+u.TargetVersion+" and verified after restart. Settings captured before and after; approved baseline preserved.")
}

func readingUptime(reading domain.Reading) (float64, bool) {
	sys, _ := reading.Status["sys"].(map[string]any)
	value, ok := sys["uptime"].(float64)
	return value, ok
}

func firmwareStepPending(reading domain.Reading, stage, target string) bool {
	sys, _ := reading.Status["sys"].(map[string]any)
	available, _ := sys["available_updates"].(map[string]any)
	release, _ := available[stage].(map[string]any)
	version, _ := release["version"].(string)
	_, pending := release["nsteps"].(float64)
	return pending && version == target
}

func firmwareChanged(before, after domain.Device) bool {
	if after.FirmwareUpdate.Active() {
		return true
	}
	beforeID, afterID := "", ""
	if before.FirmwareUpdate != nil {
		beforeID = before.FirmwareUpdate.ID
	}
	if after.FirmwareUpdate != nil {
		afterID = after.FirmwareUpdate.ID
	}
	return beforeID != afterID
}
