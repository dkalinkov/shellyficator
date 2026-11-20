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

// TCP probes can run faster than device HTTP requests.
const (
	probeConcurrency  = 24
	verifyConcurrency = 4
)

const otherNetworkHint = " If your devices are on another network or VLAN, scan it once under Advanced discovery."

func (s *Service) Discover(target string, automatic bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
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
	s.jobs.Go(func() {
		s.runDiscovery(automatic, addresses)
	})
	return nil
}

func (s *Service) runDiscovery(automatic bool, addresses []string) {
	announcementFailed := false
	single := !automatic && len(addresses) == 1
	var networks []string
	if automatic {
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		type neighborResult struct {
			addresses []string
			err       error
		}
		neighborCh := make(chan neighborResult, 1)
		go func() {
			addresses, err := s.source.Neighbors(ctx)
			neighborCh <- neighborResult{addresses, err}
		}()
		announced, err := s.source.Announcements(ctx)
		neighbors := <-neighborCh
		cancel()
		announcementFailed = err != nil
		seen := make(map[string]bool, len(announced)+len(neighbors.addresses))
		for _, address := range append(announced, neighbors.addresses...) {
			if !seen[address] {
				seen[address] = true
				addresses = append(addresses, address)
			}
		}
		networks = s.rememberedNetworks()
		open := 0
		if scan := s.networkAddresses(networks, seen); len(scan) > 0 {
			s.setProgress("Scanning remembered networks", len(scan))
			reachable := s.reachable(scan)
			open = len(reachable)
			addresses = append(addresses, reachable...)
		}
		s.logger.Info("Discovery candidates", "announced", len(announced), "neighbors", len(neighbors.addresses), "remembered_networks", len(networks), "remembered_open", open, "candidates", len(addresses), "mdns_error", err, "neighbor_error", neighbors.err)
	} else if !single {
		addresses = s.reachable(addresses)
	}
	s.setProgress("Verifying devices", len(addresses))
	limit := make(chan struct{}, verifyConcurrency)
	var group sync.WaitGroup
	foundMACs := map[string]bool{}
scan:
	for _, address := range addresses {
		select {
		case limit <- struct{}{}:
		case <-s.ctx.Done():
			break scan
		}
		group.Go(func() {
			defer func() { <-limit }()
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
					_, err = s.store.SaveReading(reading, false)
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
			if single && err != nil {
				s.discovery.Message = s.source.Explain(err)
			}
			s.mu.Unlock()
		})
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
	remembered, rememberErr := s.rememberScannedNetwork(automatic)
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
			if len(networks) == 0 {
				s.discovery.Message += otherNetworkHint
			}
		}
		if automatic && announcementFailed && s.discovery.Found > 0 {
			s.discovery.Message += " Some mDNS queries failed."
		}
	}
	if remembered != "" {
		s.discovery.Message += fmt.Sprintf(" Discover will check %s again.", remembered)
	}
	if errors.Is(rememberErr, domain.ErrNetworkLimit) {
		s.discovery.Message += fmt.Sprintf(" Not remembered: Discover already checks %d networks; forget one to add this one.", domain.MaxDiscoveryNetworks)
	}
}

func (s *Service) setProgress(stage string, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discovery.Stage, s.discovery.Total, s.discovery.Completed = stage, total, 0
}

func (s *Service) reachable(addresses []string) []string {
	open := make([]bool, len(addresses))
	limit := make(chan struct{}, probeConcurrency)
	var group sync.WaitGroup
probe:
	for i, address := range addresses {
		select {
		case limit <- struct{}{}:
		case <-s.ctx.Done():
			break probe
		}
		group.Go(func() {
			defer func() { <-limit }()
			open[i] = s.source.Reachable(s.ctx, address)
			s.mu.Lock()
			s.discovery.Completed++
			s.mu.Unlock()
		})
	}
	group.Wait()
	var result []string
	for i, ok := range open {
		if ok {
			result = append(result, addresses[i])
		}
	}
	return result
}

func (s *Service) rememberedNetworks() []string {
	if s.store == nil {
		return nil
	}
	s.inventoryMu.RLock()
	defer s.inventoryMu.RUnlock()
	networks, err := s.store.DiscoveryNetworks()
	if err != nil {
		s.logger.Warn("Could not read remembered discovery networks", "error", err)
		return nil
	}
	return networks
}

func (s *Service) networkAddresses(networks []string, seen map[string]bool) []string {
	var addresses []string
	for _, network := range networks {
		targets, err := s.source.Targets(network)
		if err != nil {
			continue
		}
		for _, address := range targets {
			if !seen[address] {
				seen[address] = true
				addresses = append(addresses, address)
			}
		}
	}
	return addresses
}

func (s *Service) rememberScannedNetwork(automatic bool) (string, error) {
	s.mu.Lock()
	target, found := s.discovery.Target, s.discovery.Found
	s.mu.Unlock()
	prefix := domain.NetworkPrefix(target)
	if automatic || found == 0 || prefix == "" || s.store == nil || s.ctx.Err() != nil {
		return "", nil
	}
	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	if err := s.store.RememberNetwork(prefix); err != nil {
		if !errors.Is(err, domain.ErrNetworkLimit) {
			s.logger.Warn("Could not remember discovery network", "network", prefix, "error", err)
		}
		return "", err
	}
	return prefix, nil
}
