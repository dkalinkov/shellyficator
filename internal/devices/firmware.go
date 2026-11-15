package devices

import (
	"context"
	"errors"
	"time"

	"shellyficator/internal/domain"
)

func (s *Service) CheckFirmware(caller context.Context, id string) (domain.Device, error) {
	if err := s.startJob(); err != nil {
		return domain.Device{}, err
	}
	defer s.jobs.Done()
	ctx, cancel := s.operationContext(caller, 40*time.Second)
	defer cancel()
	device, err := s.savedDevice(id)
	if err != nil {
		return domain.Device{}, err
	}
	reading, err := s.client.CheckForUpdate(ctx, device.Address, device.MAC)
	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	if cancelled := cancellation(ctx, caller, s.ctx); cancelled != nil {
		return domain.Device{}, cancelled
	}
	if err != nil {
		if saveErr := s.store.Failed(id, "Firmware check could not reach the saved device. Check device identity and connectivity.", errors.Is(err, domain.ErrIdentity)); saveErr != nil {
			return domain.Device{}, saveErr
		}
		return domain.Device{}, errors.Join(domain.ErrDeviceRead, err)
	}
	return s.store.SaveReading(reading, false)
}
