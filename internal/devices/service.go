// Package devices coordinates device queries and workflows independently of
// storage, device protocols, and HTTP presentation.
package devices

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"shellyficator/internal/domain"
)

// Repository supplies persistence operations in application types. Capture and
// approval are atomic repository operations, including their snapshot writes.
type Repository interface {
	Devices() ([]domain.Device, error)
	Device(string) (domain.Device, error)
	SaveReading(domain.Reading, bool) (domain.Device, error)
	ApproveReading(domain.Reading) (domain.Device, error)
	Failed(id, problem string, identityMismatch bool) error
	OrganizationOptions() (domain.OrganizationOptions, error)
	Metadata(id, room, tags, notes string) error
	Organization(id, room, tags string) error
	Snapshots(string) ([]domain.Snapshot, error)
	Baseline(domain.Device) (map[string]any, error)
	ChannelNotes(string) (map[string]string, error)
	SaveChannelNote(id, component, notes string) error
}

type Client interface {
	Read(ctx context.Context, address, expectedMAC string, configuration bool) (domain.Reading, error)
	Discover(ctx context.Context, address string) (domain.Reading, error)
}

type DiscoverySource interface {
	Announcements(context.Context) ([]string, error)
	Neighbors(context.Context) ([]string, error)
	Targets(string) ([]string, error)
	Addresses([]string) []string
	LocalNetworks() ([]domain.Network, error)
	Explain(error) string
}

var errDiscoveryUnavailable = errors.New("device discovery is not configured")

type Options struct {
	Discovery DiscoverySource
	Logger    *slog.Logger
}

type Service struct {
	store         Repository
	client        Client
	source        DiscoverySource
	logger        *slog.Logger
	announcements func(context.Context) ([]string, error)
	neighbors     func(context.Context) ([]string, error)
	ctx           context.Context
	cancel        context.CancelFunc
	jobs          sync.WaitGroup
	mu            sync.Mutex
	inventoryMu   sync.RWMutex
	closed        bool

	discovery     domain.Discovery
	refreshing    bool
	lastRefresh   time.Time
	refreshDone   chan struct{}
	refreshForced bool
	forcePending  bool
}

func New(store Repository, client Client, options Options) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	s := &Service{store: store, client: client, source: options.Discovery, logger: logger, ctx: ctx, cancel: cancel}
	if s.source != nil {
		s.announcements, s.neighbors = s.source.Announcements, s.source.Neighbors
	}
	return s
}

// startJob registers work under the same mutex that closes admission during
// shutdown, so Wait can never race a new Add after closure.
func (s *Service) startJob() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return domain.ErrClosed
	}
	s.jobs.Add(1)
	return nil
}

// Inventory reads devices and related data from one completed polling pass.
func (s *Service) Inventory() ([]domain.InventoryEntry, error) {
	if err := s.startJob(); err != nil {
		return nil, err
	}
	defer s.jobs.Done()
	s.inventoryMu.RLock()
	defer s.inventoryMu.RUnlock()
	devices, err := s.store.Devices()
	if err != nil {
		return nil, err
	}
	result := make([]domain.InventoryEntry, 0, len(devices))
	for _, device := range devices {
		entry, err := s.entry(device)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, nil
}

func (s *Service) entry(device domain.Device) (domain.InventoryEntry, error) {
	baseline, err := s.store.Baseline(device)
	if err != nil {
		return domain.InventoryEntry{}, err
	}
	notes, err := s.store.ChannelNotes(device.ID)
	if err != nil {
		return domain.InventoryEntry{}, err
	}
	return domain.InventoryEntry{Device: device, Baseline: baseline, ChannelNotes: notes}, nil
}

func (s *Service) Detail(id string) (domain.DeviceDetails, error) {
	if err := s.startJob(); err != nil {
		return domain.DeviceDetails{}, err
	}
	defer s.jobs.Done()
	s.inventoryMu.RLock()
	defer s.inventoryMu.RUnlock()
	device, err := s.store.Device(id)
	if err != nil {
		return domain.DeviceDetails{}, err
	}
	entry, err := s.entry(device)
	if err != nil {
		return domain.DeviceDetails{}, err
	}
	snapshots, err := s.store.Snapshots(id)
	if err != nil {
		return domain.DeviceDetails{}, err
	}
	options, err := s.store.OrganizationOptions()
	if err != nil {
		return domain.DeviceDetails{}, err
	}
	return domain.DeviceDetails{InventoryEntry: entry, Snapshots: snapshots, OrganizationOptions: options}, nil
}

func (s *Service) Discovery() domain.Discovery {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.discovery
}

func (s *Service) LocalNetworks() ([]domain.Network, error) {
	if err := s.startJob(); err != nil {
		return nil, err
	}
	defer s.jobs.Done()
	if s.source == nil {
		return nil, errDiscoveryUnavailable
	}
	return s.source.LocalNetworks()
}

func (s *Service) OrganizationOptions() (domain.OrganizationOptions, error) {
	if err := s.startJob(); err != nil {
		return domain.OrganizationOptions{}, err
	}
	defer s.jobs.Done()
	s.inventoryMu.RLock()
	defer s.inventoryMu.RUnlock()
	return s.store.OrganizationOptions()
}

func (s *Service) Metadata(id, room, tags, notes string) error {
	return s.change(func() error { return s.store.Metadata(id, room, tags, notes) })
}

func (s *Service) Organization(id, room, tags string) error {
	return s.change(func() error { return s.store.Organization(id, room, tags) })
}

func (s *Service) SaveChannelNote(id, component, notes string) error {
	return s.change(func() error { return s.store.SaveChannelNote(id, component, notes) })
}

func (s *Service) change(apply func() error) error {
	if err := s.startJob(); err != nil {
		return err
	}
	defer s.jobs.Done()
	s.inventoryMu.Lock()
	defer s.inventoryMu.Unlock()
	if s.ctx.Err() != nil {
		return domain.ErrClosed
	}
	return apply()
}

// Close cancels device work and drains it before the caller tears down storage.
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.jobs.Wait()
}
