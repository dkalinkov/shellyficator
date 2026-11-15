package devices

import (
	"context"
	"errors"
	"time"

	"shellyficator/internal/domain"
)

// operationContext joins caller cancellation with service shutdown. Its cleanup
// stops the shutdown callback and releases the timeout's resources.
func (s *Service) operationContext(caller context.Context, timeout time.Duration) (context.Context, func()) {
	ctx, cancel := context.WithTimeout(caller, timeout)
	stop := context.AfterFunc(s.ctx, cancel)
	return ctx, func() { stop(); cancel() }
}

func (s *Service) savedDevice(id string) (domain.Device, error) {
	s.inventoryMu.RLock()
	defer s.inventoryMu.RUnlock()
	return s.store.Device(id)
}

// RefreshDevice reads status without creating snapshots or selecting a baseline.
func (s *Service) RefreshDevice(caller context.Context, id string) (domain.RefreshResult, error) {
	if err := s.startJob(); err != nil {
		return domain.RefreshResult{}, err
	}
	defer s.jobs.Done()
	ctx, cancel := s.operationContext(caller, 12*time.Second)
	defer cancel()
	device, err := s.savedDevice(id)
	if err != nil {
		return domain.RefreshResult{}, err
	}
	if device.FirmwareUpdate.Active() {
		return domain.RefreshResult{}, domain.ErrFirmwareBusy
	}
	reading, err := s.client.Read(ctx, device.Address, device.MAC, false)
	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	if err := cancellation(ctx, caller, s.ctx); err != nil {
		return domain.RefreshResult{}, err
	}
	latest, lookupErr := s.store.Device(id)
	if lookupErr == nil && firmwareChanged(device, latest) {
		return domain.RefreshResult{}, domain.ErrFirmwareBusy
	}
	if err != nil {
		if saveErr := s.store.Failed(id, "Status could not be read. Check device identity and connectivity.", errors.Is(err, domain.ErrIdentity)); saveErr != nil {
			return domain.RefreshResult{}, saveErr
		}
		return domain.RefreshResult{}, errors.Join(domain.ErrDeviceRead, err)
	}
	// Recheck the saved configuration under the publication lock: a concurrent
	// capture may have recovered it while this status request was in flight.
	device, err = s.store.Device(id)
	if err != nil {
		return domain.RefreshResult{}, err
	}
	// A status-only refresh retains an unavailable-configuration warning.
	if !device.ConfigAvailable && reading.Problem == "" {
		reading.Problem = "Configuration is unavailable. Capture config to retry."
	}
	if _, err := s.store.SaveReading(reading, false); err != nil {
		return domain.RefreshResult{}, err
	}
	return domain.RefreshResult{StatusAvailable: reading.Status != nil}, nil
}

func (s *Service) Capture(caller context.Context, id string) (domain.Device, error) {
	return s.configuration(caller, id, false)
}

func (s *Service) ApproveBaseline(caller context.Context, id string) (domain.Device, error) {
	return s.configuration(caller, id, true)
}

func (s *Service) configuration(caller context.Context, id string, approve bool) (domain.Device, error) {
	if err := s.startJob(); err != nil {
		return domain.Device{}, err
	}
	defer s.jobs.Done()
	ctx, cancel := s.operationContext(caller, 20*time.Second)
	defer cancel()
	device, err := s.savedDevice(id)
	if err != nil {
		return domain.Device{}, err
	}
	if device.FirmwareUpdate.Active() {
		return domain.Device{}, domain.ErrFirmwareBusy
	}
	reading, err := s.client.Read(ctx, device.Address, device.MAC, true)
	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	if err := cancellation(ctx, caller, s.ctx); err != nil {
		return domain.Device{}, err
	}
	latest, lookupErr := s.store.Device(id)
	if lookupErr == nil && firmwareChanged(device, latest) {
		return domain.Device{}, domain.ErrFirmwareBusy
	}
	if err != nil {
		if saveErr := s.store.Failed(id, "Configuration could not be captured. Check device identity and connectivity.", errors.Is(err, domain.ErrIdentity)); saveErr != nil {
			return domain.Device{}, saveErr
		}
		return domain.Device{}, errors.Join(domain.ErrDeviceRead, err)
	}
	if reading.Config == nil {
		// Identity/status can still be valid when the configuration request fails.
		// Save those observations without adding a snapshot or changing baseline.
		if _, err := s.store.SaveReading(reading, false); err != nil {
			return domain.Device{}, err
		}
		return domain.Device{}, domain.ErrConfigUnavailable
	}
	if approve {
		device, err := s.store.ApproveReading(reading)
		if err != nil {
			return domain.Device{}, errors.Join(domain.ErrBaselineSave, err)
		}
		return device, nil
	}
	return s.store.SaveReading(reading, true)
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
