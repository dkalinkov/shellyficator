package devices

import (
	"context"
	"errors"
	"sync"
	"time"

	"shellyficator/internal/domain"
)

func (s *Service) Refresh(force bool) <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return nil
	}
	if s.refreshDone != nil {
		if force && !s.refreshForced {
			s.forcePending = true
		}
		return s.refreshDone
	}
	if !force && time.Since(s.lastRefresh) < 30*time.Second {
		return nil
	}
	s.refreshForced = force
	s.lastRefresh = time.Now()
	s.refreshDone = make(chan struct{})
	done := s.refreshDone
	s.jobs.Go(func() {
		for {
			s.refreshDevices(force)
			s.mu.Lock()
			if s.ctx.Err() == nil && s.forcePending {
				s.forcePending = false
				s.refreshForced = true
				force = true
				s.lastRefresh = time.Now()
				s.mu.Unlock()
				continue
			}
			s.refreshDone, s.refreshForced, s.forcePending = nil, false, false
			close(done)
			s.mu.Unlock()
			return
		}
	})
	return done
}

func (s *Service) refreshDevices(force bool) {
	if s.ctx.Err() != nil {
		return
	}
	s.retryFirmwareResults()
	s.inventoryMu.RLock()
	records, err := s.store.Devices()
	s.inventoryMu.RUnlock()
	if err != nil {
		s.logger.Error("Could not load devices for refresh", "error", err)
		return
	}
	limit := make(chan struct{}, 4)
	var group sync.WaitGroup
	results := make([]struct {
		reading domain.Reading
		err     error
		skipped bool
	}, len(records))
	for i, record := range records {
		if record.FirmwareUpdate.Active() {
			results[i].skipped = true
			continue
		}
		select {
		case limit <- struct{}{}:
		case <-s.ctx.Done():
			group.Wait()
			return
		}
		group.Go(func() {
			defer func() { <-limit }()
			lastRead := record.LastCaptured
			readConfig := force || !record.Reachable || record.ConsecutiveFailures > 0 || !record.ConfigAvailable || time.Since(lastRead) >= 5*time.Minute
			timeout := 12 * time.Second
			if readConfig {
				timeout = 20 * time.Second
			}
			ctx, cancel := context.WithTimeout(s.ctx, timeout)
			defer cancel()
			results[i].reading, results[i].err = s.client.Read(ctx, record.Address, record.MAC, readConfig)
		})
	}
	group.Wait()
	// Publish the full pass under one lock so readers never see partial results.
	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	if s.ctx.Err() != nil {
		return
	}
	for i, result := range results {
		if result.skipped {
			continue
		}
		record := records[i]
		if _, err := s.currentRead(record); err != nil {
			if !errors.Is(err, domain.ErrDeviceMoved) && !errors.Is(err, domain.ErrFirmwareBusy) && !errors.Is(err, domain.ErrNotFound) {
				s.logger.Error("Could not confirm device for refresh", "device", record.ID, "error", err)
			}
			continue
		}
		if result.err != nil {
			problem := "Device could not be reached. Last successful readings are preserved."
			if errors.Is(result.err, domain.ErrIdentity) {
				problem = "Address now belongs to a different device. Rediscover this device."
			}
			if saveErr := s.store.Failed(record.ID, problem, errors.Is(result.err, domain.ErrIdentity)); saveErr != nil {
				s.logger.Error("Could not save device check failure", "device", record.ID, "error", saveErr)
			}
			continue
		}
		if _, err := s.store.SaveReading(result.reading, false); err != nil {
			s.logger.Error("Could not save device reading", "device", record.ID, "error", err)
		}
	}
}
