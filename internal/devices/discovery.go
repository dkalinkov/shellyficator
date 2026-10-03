package devices

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"shellyficator/internal/domain"
)

func (s *Service) Discover(target string, automatic bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return domain.ErrClosed
	}
	if s.discovery.Running {
		return errors.New("Discovery is already running.")
	}
	if s.source == nil {
		return errDiscoveryUnavailable
	}
	target = strings.TrimSpace(target)
	var addresses []string
	if !automatic {
		var err error
		addresses, err = s.source.Targets(target)
		if err != nil {
			return err
		}
	} else {
		target = "network announcements"
	}
	s.discovery = domain.Discovery{Running: true, Total: len(addresses), Target: target, Stage: "Scanning addresses"}
	if automatic {
		s.discovery.Stage = "Finding devices"
	}
	s.jobs.Add(1)
	go func() {
		defer s.jobs.Done()
		s.runDiscovery(automatic, addresses)
	}()
	return nil
}

func (s *Service) runDiscovery(automatic bool, addresses []string) {
	announcementFailed := false
	if automatic {
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		type neighborResult struct {
			addresses []string
			err       error
		}
		neighborCh := make(chan neighborResult, 1)
		go func() {
			addresses, err := s.neighbors(ctx)
			neighborCh <- neighborResult{addresses, err}
		}()
		announced, err := s.announcements(ctx)
		neighbors := <-neighborCh
		cancel()
		announcementFailed = err != nil
		addresses = s.source.Addresses(append(announced, neighbors.addresses...))
		if s.logger != nil {
			s.logger.Info("Discovery candidates", "announced", len(announced), "neighbors", len(neighbors.addresses), "candidates", len(addresses), "mdns_error", err, "neighbor_error", neighbors.err)
		}
		s.mu.Lock()
		s.discovery.Total = len(addresses)
		s.discovery.Stage = "Verifying devices"
		s.mu.Unlock()
	}
	limit := make(chan struct{}, 4)
	var group sync.WaitGroup
	foundMACs := map[string]bool{}
scan:
	for _, address := range addresses {
		select {
		case limit <- struct{}{}:
		case <-s.ctx.Done():
			break scan
		}
		group.Add(1)
		go func(address string) {
			defer func() { <-limit; group.Done() }()
			ctx, cancel := context.WithTimeout(s.ctx, 12*time.Second)
			defer cancel()
			reading, err := s.client.Discover(ctx, address)
			if s.ctx.Err() != nil {
				return
			}
			found := false
			if err == nil {
				s.inventoryMu.Lock()
				if s.ctx.Err() == nil {
					_, err = s.store.SaveReading(reading, true)
					found = err == nil
				}
				s.inventoryMu.Unlock()
			}
			s.mu.Lock()
			s.discovery.Completed++
			if found && !foundMACs[reading.Identity.MAC] {
				foundMACs[reading.Identity.MAC] = true
				s.discovery.Found++
			}
			if !automatic && len(addresses) == 1 && err != nil {
				s.discovery.Message = s.source.Explain(err)
			}
			s.mu.Unlock()
		}(address)
		if !automatic {
			timer := time.NewTimer(50 * time.Millisecond)
			select {
			case <-timer.C:
			case <-s.ctx.Done():
				timer.Stop()
				break scan
			}
		}
	}
	group.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discovery.Running = false
	if s.ctx.Err() != nil {
		s.discovery.Message = "Discovery cancelled."
		return
	}
	if s.discovery.Message == "" {
		s.discovery.Message = fmt.Sprintf("Devices found: %d.", s.discovery.Found)
		if automatic && s.discovery.Found == 0 {
			s.discovery.Message = fmt.Sprintf("No supported devices responded at %d discovered addresses.", len(addresses))
			if len(addresses) == 0 {
				s.discovery.Message = "No Shelly announcements received. Check multicast access."
				if announcementFailed {
					s.discovery.Message = "mDNS query failed. Check multicast access."
				}
			}
		}
		if automatic && announcementFailed && s.discovery.Found > 0 {
			s.discovery.Message += " Some mDNS queries failed."
		}
	}
}
