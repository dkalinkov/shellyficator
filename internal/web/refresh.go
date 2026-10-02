package web

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/shelly"
)

func (s *Server) refresh(force bool) <-chan struct{} {
	s.mu.Lock()
	if s.refreshing {
		// A manual request during status polling gets a configuration pass too.
		if force && !s.refreshForced {
			s.refreshForcePending = true
		}
		done := s.refreshDone
		s.mu.Unlock()
		return done
	}
	if !force && time.Since(s.lastRefresh) < 30*time.Second {
		s.mu.Unlock()
		return nil
	}
	s.refreshing = true
	s.refreshForced = force
	s.lastRefresh = time.Now()
	s.refreshDone = make(chan struct{})
	done := s.refreshDone
	s.mu.Unlock()
	go func() {
		for {
			s.refreshDevices(force)
			s.mu.Lock()
			if s.refreshForcePending {
				s.refreshForcePending = false
				s.refreshForced = true
				force = true
				s.lastRefresh = time.Now()
				s.mu.Unlock()
				continue
			}
			s.refreshing = false
			s.refreshForced = false
			close(done)
			s.mu.Unlock()
			return
		}
	}()
	return done
}

func (s *Server) refreshDevices(force bool) {
	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	records, err := s.store.Devices()
	if err != nil {
		s.store.App.Logger().Error("Could not load devices for refresh", "error", err)
		return
	}
	limit := make(chan struct{}, 4)
	var group sync.WaitGroup
	for _, record := range records {
		limit <- struct{}{}
		group.Add(1)
		go func(record *core.Record) {
			defer func() { <-limit; group.Done() }()
			lastRead := record.GetDateTime("last_captured").Time()
			readConfig := force || !record.GetBool("reachable") || record.GetInt("consecutive_failures") > 0 || !record.GetBool("config_available") || time.Since(lastRead) >= 5*time.Minute
			timeout := 12 * time.Second
			if readConfig {
				timeout = 20 * time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			reading, err := s.client.Read(ctx, record.GetString("address"), record.GetString("mac"), readConfig)
			if err != nil {
				problem := "Device could not be reached. Last successful readings are preserved."
				if errors.Is(err, shelly.ErrIdentity) {
					problem = "Address now belongs to a different device. Rediscover this device."
				}
				if saveErr := s.store.Failed(record.Id, problem, errors.Is(err, shelly.ErrIdentity)); saveErr != nil {
					s.store.App.Logger().Error("Could not save device check failure", "device", record.Id)
				}
				return
			}
			if _, err := s.store.SaveReading(reading, false); err != nil {
				s.store.App.Logger().Error("Could not save device reading", "device", record.Id)
			}
		}(record)
	}
	group.Wait()
}
