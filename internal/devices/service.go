package devices

import (
	"context"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/inventory"
	"shellyficator/internal/shelly"
)

type Device struct {
	Record       *core.Record
	Baseline     map[string]any
	ChannelNotes map[string]string
}

type Discovery struct {
	Running                 bool
	Total, Completed, Found int
	Target, Message         string
	Stage                   string
}

type Service struct {
	store         *inventory.Store
	client        *shelly.Client
	announcements func(context.Context) ([]string, error)
	neighbors     func(context.Context) ([]string, error)
	ctx           context.Context
	cancel        context.CancelFunc
	jobs          sync.WaitGroup
	mu            sync.Mutex
	inventoryMu   sync.RWMutex
	closed        bool
	discovery     Discovery
	refreshing    bool
	lastRefresh   time.Time
	refreshDone   chan struct{}
	refreshForced bool
	forcePending  bool
}

func New(store *inventory.Store, client *shelly.Client) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{store: store, client: client, announcements: shelly.Announcements, neighbors: shelly.Neighbors, ctx: ctx, cancel: cancel}
}

// Inventory reads records and their related presentation data from one completed polling pass.
func (s *Service) Inventory() ([]Device, error) {
	s.inventoryMu.RLock()
	defer s.inventoryMu.RUnlock()
	records, err := s.store.Devices()
	if err != nil {
		return nil, err
	}
	result := make([]Device, 0, len(records))
	for _, record := range records {
		result = append(result, Device{Record: record, Baseline: s.store.Baseline(record), ChannelNotes: s.store.ChannelNotes(record.Id)})
	}
	return result, nil
}

func (s *Service) Discovery() Discovery {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.discovery
}

// Close cancels device work and drains it before the caller tears down storage.
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.jobs.Wait()
}
