package web

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/shelly"
)

type Discovery struct {
	Running                 bool
	Total, Completed, Found int
	Target, Message         string
	Stage                   string
}

func (s *Server) progress(e *core.RequestEvent) error {
	s.mu.Lock()
	job := s.discovery
	s.mu.Unlock()
	return s.render(e, "discovery", Page{Discovery: job})
}

func (s *Server) discover(e *core.RequestEvent) error {
	target := strings.TrimSpace(e.Request.FormValue("target"))
	automatic := e.Request.FormValue("mode") == "auto"
	var addresses []string
	if !automatic {
		var err error
		addresses, err = shelly.Targets(target)
		if err != nil {
			return s.redirect(e, "/", err.Error())
		}
	} else {
		target = "network announcements"
	}
	s.mu.Lock()
	if s.discovery.Running {
		s.mu.Unlock()
		return s.redirect(e, "/", "Discovery is already running.")
	}
	s.discovery = Discovery{Running: true, Total: len(addresses), Target: target, Stage: "Scanning addresses"}
	if automatic {
		s.discovery.Stage = "Finding devices"
	}
	s.mu.Unlock()
	go s.runDiscovery(automatic, addresses)
	return s.redirect(e, "/", "")
}

func (s *Server) runDiscovery(automatic bool, addresses []string) {
	announcementFailed := false
	if automatic {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		type neighborResult struct {
			addresses []string
			err       error
		}
		neighborCh := make(chan neighborResult, 1)
		go func() {
			if s.neighbors == nil {
				neighborCh <- neighborResult{}
				return
			}
			addresses, err := s.neighbors(ctx)
			neighborCh <- neighborResult{addresses, err}
		}()
		announced, err := s.announcements(ctx)
		neighbors := <-neighborCh
		cancel()
		announcementFailed = err != nil
		addresses = shelly.DiscoveryAddresses(append(announced, neighbors.addresses...))
		if s.store != nil {
			s.store.App.Logger().Info("Discovery candidates", "announced", len(announced), "neighbors", len(neighbors.addresses), "candidates", len(addresses), "mdns_error", err, "neighbor_error", neighbors.err)
		}
		s.mu.Lock()
		s.discovery.Total = len(addresses)
		s.discovery.Stage = "Verifying devices"
		s.mu.Unlock()
	}
	limit := make(chan struct{}, 4)
	var group sync.WaitGroup
	foundMACs := map[string]bool{}
	for _, address := range addresses {
		limit <- struct{}{}
		group.Add(1)
		go func(address string) {
			defer func() { <-limit; group.Done() }()
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			reading, err := s.client.Discover(ctx, address)
			found := false
			if err == nil {
				_, err = s.store.SaveReading(reading, true)
				found = err == nil
			}
			s.mu.Lock()
			s.discovery.Completed++
			if found && !foundMACs[reading.Identity.MAC] {
				foundMACs[reading.Identity.MAC] = true
				s.discovery.Found++
			}
			if !automatic && len(addresses) == 1 && err != nil {
				s.discovery.Message = shelly.DiscoveryError(err)
			}
			s.mu.Unlock()
		}(address)
		if !automatic {
			time.Sleep(50 * time.Millisecond)
		}
	}
	group.Wait()
	s.mu.Lock()
	s.discovery.Running = false
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
	s.mu.Unlock()
}
