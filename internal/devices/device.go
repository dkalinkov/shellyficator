package devices

import (
	"context"
	"errors"
	"time"

	"shellyficator/internal/domain"
)

func (s *Service) operationContext(caller context.Context, timeout time.Duration) (context.Context, func()) {
	ctx, cancel := context.WithTimeout(caller, timeout)
	stop := context.AfterFunc(s.ctx, cancel)
	return ctx, func() { stop(); cancel() }
}

func (s *Service) savedDevice(id string) (domain.Device, error) {
	s.retryFirmwareResults()
	s.inventoryMu.RLock()
	defer s.inventoryMu.RUnlock()
	return s.store.Device(id)
}

// currentRead checks that a read still applies while the caller holds inventoryMu.
func (s *Service) currentRead(before domain.Device) (domain.Device, error) {
	latest, err := s.store.Device(before.ID)
	if err != nil {
		return domain.Device{}, err
	}
	if firmwareChanged(before, latest) {
		return domain.Device{}, domain.ErrFirmwareBusy
	}
	if deviceMoved(before, latest) {
		return domain.Device{}, domain.ErrDeviceMoved
	}
	return latest, nil
}

func deviceMoved(before, after domain.Device) bool {
	return before.Address != after.Address || before.MAC != after.MAC
}

type deviceRead uint8

const (
	readStatus deviceRead = iota
	captureConfig
	approveConfig
	checkFirmware
)

func (s *Service) RefreshDevice(caller context.Context, id string) (bool, error) {
	device, err := s.readAndSave(caller, id, readStatus)
	return err == nil && device.StatusAvailable, err
}

func (s *Service) Capture(caller context.Context, id string) (domain.Device, error) {
	return s.readAndSave(caller, id, captureConfig)
}

func (s *Service) ApproveBaseline(caller context.Context, id string) (domain.Device, error) {
	return s.readAndSave(caller, id, approveConfig)
}

func (s *Service) CheckFirmware(caller context.Context, id string) (domain.Device, error) {
	return s.readAndSave(caller, id, checkFirmware)
}

func (s *Service) readAndSave(caller context.Context, id string, mode deviceRead) (domain.Device, error) {
	if err := s.startJob(); err != nil {
		return domain.Device{}, err
	}
	defer s.jobs.Done()

	timeout, problem := 20*time.Second, "Configuration could not be captured. Check device identity and connectivity."
	switch mode {
	case readStatus:
		timeout, problem = 12*time.Second, "Status could not be read. Check device identity and connectivity."
	case checkFirmware:
		timeout, problem = 40*time.Second, "Firmware check could not reach the saved device. Check device identity and connectivity."
	}
	ctx, cancel := s.operationContext(caller, timeout)
	defer cancel()
	device, err := s.savedDevice(id)
	if err != nil {
		return domain.Device{}, err
	}
	if device.FirmwareUpdate.Active() {
		return domain.Device{}, domain.ErrFirmwareBusy
	}

	var reading domain.Reading
	if mode == checkFirmware {
		reading, err = s.client.CheckForUpdate(ctx, device.Address, device.MAC)
	} else {
		reading, err = s.client.Read(ctx, device.Address, device.MAC, mode != readStatus)
	}

	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	if err := cancellation(ctx, caller, s.ctx); err != nil {
		return domain.Device{}, err
	}
	latest, lookupErr := s.currentRead(device)
	if lookupErr != nil {
		return domain.Device{}, lookupErr
	}
	if err != nil {
		if saveErr := s.store.Failed(id, problem, errors.Is(err, domain.ErrIdentity)); saveErr != nil {
			return domain.Device{}, saveErr
		}
		return domain.Device{}, errors.Join(domain.ErrDeviceRead, err)
	}

	if mode == readStatus && !latest.ConfigAvailable && reading.Problem == "" {
		reading.Problem = "Configuration is unavailable. Capture config to retry."
	}
	if (mode == captureConfig || mode == approveConfig) && reading.Config == nil {
		if _, err := s.store.SaveReading(reading, false); err != nil {
			return domain.Device{}, err
		}
		return domain.Device{}, domain.ErrConfigUnavailable
	}
	if mode == approveConfig {
		device, err := s.store.ApproveReading(reading)
		if err != nil {
			return domain.Device{}, errors.Join(domain.ErrBaselineSave, err)
		}
		return device, nil
	}
	return s.store.SaveReading(reading, mode == captureConfig)
}

// Cancellation by the caller or shutdown does not report a device outage. A
// service-imposed read deadline still represents a failed device check.
func cancellation(operation, caller, lifetime context.Context) error {
	if err := caller.Err(); err != nil {
		return err
	}
	if lifetime.Err() != nil {
		return domain.ErrClosed
	}
	if errors.Is(operation.Err(), context.Canceled) {
		return operation.Err()
	}
	return nil
}
