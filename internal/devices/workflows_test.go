package devices

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"shellyficator/internal/domain"
)

type memoryRepository struct {
	Repository
	mu                       sync.Mutex
	devices                  map[string]domain.Device
	snapshots                map[string][]domain.Snapshot
	baselines                map[string]map[string]any
	devicesErr, deviceErr    error
	baselineErr, notesErr    error
	snapshotsErr, optionsErr error
	saveErr, failedErr       error
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		devices: map[string]domain.Device{}, snapshots: map[string][]domain.Snapshot{},
		baselines: map[string]map[string]any{},
	}
}

func (r *memoryRepository) Devices() ([]domain.Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var devices []domain.Device
	for _, device := range r.devices {
		devices = append(devices, device)
	}
	return devices, r.devicesErr
}

func (r *memoryRepository) Device(id string) (domain.Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deviceErr != nil {
		return domain.Device{}, r.deviceErr
	}
	device, found := r.devices[id]
	if !found {
		return domain.Device{}, domain.ErrNotFound
	}
	return device, nil
}

func (r *memoryRepository) SaveReading(reading domain.Reading, capture bool) (domain.Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveErr != nil {
		return domain.Device{}, r.saveErr
	}
	var device domain.Device
	for _, candidate := range r.devices {
		if candidate.MAC == reading.Identity.MAC {
			device = candidate
			break
		}
	}
	if device.ID == "" {
		device.ID = reading.Identity.ID
	}
	device = device.ApplyReading(reading, capture, time.Now())
	if capture && reading.Config != nil {
		snapshot := domain.Snapshot{ID: fmt.Sprintf("snapshot-%d", len(r.snapshots[device.ID])+1)}
		r.snapshots[device.ID] = append(r.snapshots[device.ID], snapshot)
	}
	r.devices[device.ID] = device
	return device, nil
}

func (r *memoryRepository) ApproveReading(domain.Reading) (domain.Device, error) {
	return domain.Device{}, r.saveErr
}

func (r *memoryRepository) Failed(id, problem string, mismatch bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failedErr != nil {
		return r.failedErr
	}
	r.devices[id] = r.devices[id].FailedCheck(problem, mismatch, time.Now())
	return nil
}

func (r *memoryRepository) Baseline(device domain.Device) (map[string]any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.baselines[device.ID], r.baselineErr
}

func (r *memoryRepository) ChannelNotes(string) (map[string]string, error) {
	return nil, r.notesErr
}

func (r *memoryRepository) Snapshots(id string) ([]domain.Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshots[id], r.snapshotsErr
}

func (r *memoryRepository) OrganizationOptions() (domain.OrganizationOptions, error) {
	devices, err := r.Devices()
	if err != nil {
		return domain.OrganizationOptions{}, err
	}
	return domain.Options(devices), r.optionsErr
}

type workflowClient struct {
	read     func(context.Context, string, string, bool) (domain.Reading, error)
	discover func(context.Context, string) (domain.Reading, error)
	update   func(context.Context, string, string, string) error
}

func (c workflowClient) StartFirmwareUpdate(ctx context.Context, address, mac, stage string) error {
	if c.update == nil {
		return domain.ErrFirmwareRejected
	}
	return c.update(ctx, address, mac, stage)
}

func (r *memoryRepository) SaveFirmwareUpdates(updates map[string]domain.FirmwareUpdate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saveErr != nil {
		return r.saveErr
	}
	for id, update := range updates {
		device := r.devices[id]
		copy := update
		device.FirmwareUpdate = &copy
		r.devices[id] = device
	}
	return nil
}

func (c workflowClient) Read(ctx context.Context, address, mac string, config bool) (domain.Reading, error) {
	return c.read(ctx, address, mac, config)
}

func (c workflowClient) Discover(ctx context.Context, address string) (domain.Reading, error) {
	return c.discover(ctx, address)
}

func (c workflowClient) CheckForUpdate(ctx context.Context, address, mac string) (domain.Reading, error) {
	return c.read(ctx, address, mac, false)
}

func workflowFixture(t *testing.T, client workflowClient) (*Service, *memoryRepository, domain.Device) {
	t.Helper()
	repository := newMemoryRepository()
	device := domain.Device{
		ID: "saved-device", DeviceID: "reported-device", MAC: "AABBCCDDEEFF", Address: "192.0.2.1",
		Room: "Office", Tags: []string{"lamp"}, Notes: "Keep these notes", BaselineID: "approved-before",
		Reachable: true, StatusAvailable: true, ConfigAvailable: true, LastSeen: time.Now(), LastCaptured: time.Now(),
		Status: map[string]any{"switch:0": map[string]any{"output": false}},
		Config: map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}},
	}
	repository.devices[device.ID] = device
	repository.baselines[device.ID] = device.Config
	service := New(repository, client, Options{})
	t.Cleanup(service.Close)
	return service, repository, device
}

func deviceReading(device domain.Device) domain.Reading {
	return domain.Reading{
		Identity: domain.Identity{ID: device.DeviceID, MAC: device.MAC}, Address: device.Address,
		Status: map[string]any{"switch:0": map[string]any{"output": true}},
		Config: map[string]any{"switch:0": map[string]any{"in_mode": "detached"}}, ConfigAttempted: true,
	}
}

func TestServiceQueriesPropagateRelatedDataFailures(t *testing.T) {
	failure := errors.New("storage read failed")
	for _, test := range []struct {
		name   string
		set    func(*memoryRepository)
		detail bool
	}{
		{"inventory devices", func(r *memoryRepository) { r.devicesErr = failure }, false},
		{"inventory baseline", func(r *memoryRepository) { r.baselineErr = failure }, false},
		{"inventory notes", func(r *memoryRepository) { r.notesErr = failure }, false},
		{"detail device", func(r *memoryRepository) { r.deviceErr = failure }, true},
		{"detail snapshots", func(r *memoryRepository) { r.snapshotsErr = failure }, true},
		{"detail organization", func(r *memoryRepository) { r.optionsErr = failure }, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, repository, device := workflowFixture(t, workflowClient{})
			test.set(repository)
			var err error
			if test.detail {
				_, err = service.Detail(device.ID)
			} else {
				_, err = service.Inventory()
			}
			if !errors.Is(err, failure) {
				t.Fatalf("storage failure was hidden: %v", err)
			}
		})
	}
}

func TestCaptureWithoutSettingsKeepsSavedConfigAndUpdatesStatus(t *testing.T) {
	var readDevice domain.Device
	s, repository, before := workflowFixture(t, workflowClient{read: func(context.Context, string, string, bool) (domain.Reading, error) {
		reading := deviceReading(readDevice)
		reading.Config, reading.Problem = nil, "settings unavailable"
		return reading, nil
	}})
	readDevice = before
	if _, err := s.Capture(t.Context(), before.ID); !errors.Is(err, domain.ErrConfigUnavailable) {
		t.Fatalf("capture lost the configuration error: %v", err)
	}
	after, _ := repository.Device(before.ID)
	snapshots, _ := repository.Snapshots(before.ID)
	if after.ConfigAvailable || !after.StatusCurrent() || after.BaselineID != before.BaselineID || after.LastCaptured != before.LastCaptured ||
		after.Config["switch:0"].(map[string]any)["in_mode"] != "momentary" || after.Status["switch:0"].(map[string]any)["output"] != true || len(snapshots) != 0 {
		t.Fatal("capture changed history or saved settings, or discarded fresh status")
	}
}

func TestStatusRefreshKeepsTheSavedConfigurationWarning(t *testing.T) {
	var readDevice domain.Device
	s, repository, device := workflowFixture(t, workflowClient{read: func(context.Context, string, string, bool) (domain.Reading, error) {
		reading := deviceReading(readDevice)
		reading.Config, reading.ConfigAttempted = nil, false
		return reading, nil
	}})
	readDevice = device
	device.ConfigAvailable = false
	repository.devices[device.ID] = device
	available, err := s.RefreshDevice(t.Context(), device.ID)
	after, _ := repository.Device(device.ID)
	if err != nil || !available || after.ConfigAvailable || after.Problem != "Configuration is unavailable. Capture config to retry." {
		t.Fatalf("status refresh lost the settings warning: device=%+v err=%v", after, err)
	}
}

func TestDeviceReadReportsFailureToSaveHealth(t *testing.T) {
	s, repository, device := workflowFixture(t, workflowClient{read: func(context.Context, string, string, bool) (domain.Reading, error) {
		return domain.Reading{}, domain.ErrIdentity
	}})
	storageFailure := errors.New("cannot persist failure")
	repository.failedErr = storageFailure
	_, err := s.RefreshDevice(t.Context(), device.ID)
	if !errors.Is(err, storageFailure) || errors.Is(err, domain.ErrDeviceRead) {
		t.Fatalf("failure to save device health was hidden: %v", err)
	}
}

func TestSingleDeviceCancellationLeavesSavedInventoryAvailable(t *testing.T) {
	for _, callerCancels := range []bool{true, false} {
		t.Run(fmt.Sprintf("caller=%t", callerCancels), func(t *testing.T) {
			started := make(chan struct{})
			s, repository, before := workflowFixture(t, workflowClient{read: func(ctx context.Context, address, mac string, config bool) (domain.Reading, error) {
				close(started)
				<-ctx.Done()
				return domain.Reading{}, ctx.Err()
			}})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			completed := make(chan error, 1)
			go func() {
				if callerCancels {
					_, err := s.RefreshDevice(ctx, before.ID)
					completed <- err
				} else {
					_, err := s.ApproveBaseline(ctx, before.ID)
					completed <- err
				}
			}()
			waitForJob(t, started)
			inventoryDone := make(chan error, 1)
			go func() { _, err := s.Inventory(); inventoryDone <- err }()
			select {
			case err := <-inventoryDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("inventory waited for the device read")
			}
			want := domain.ErrClosed
			if callerCancels {
				cancel()
				want = context.Canceled
			} else {
				closeService(t, s)
			}
			select {
			case err := <-completed:
				if !errors.Is(err, want) || errors.Is(err, domain.ErrDeviceRead) {
					t.Fatalf("cancellation error=%v want=%v", err, want)
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled device read stayed active")
			}
			after, _ := repository.Device(before.ID)
			if after.LastChecked != before.LastChecked || after.BaselineID != before.BaselineID {
				t.Fatal("cancellation changed health or the approved baseline")
			}
		})
	}
}

func TestApprovalSaveFailurePreservesCause(t *testing.T) {
	var readDevice domain.Device
	s, repository, device := workflowFixture(t, workflowClient{read: func(context.Context, string, string, bool) (domain.Reading, error) {
		return deviceReading(readDevice), nil
	}})
	readDevice = device
	failure := errors.New("transaction failed")
	repository.saveErr = failure
	_, err := s.ApproveBaseline(t.Context(), device.ID)
	if !errors.Is(err, domain.ErrBaselineSave) || !errors.Is(err, failure) || errors.Is(err, domain.ErrDeviceRead) {
		t.Fatalf("approval failure lost its type or cause: %v", err)
	}
}

func TestClosedServiceRejectsQueriesAndSynchronousWork(t *testing.T) {
	service, _, device := workflowFixture(t, workflowClient{})
	service.Close()
	operations := []func() error{
		func() error { _, err := service.Inventory(); return err },
		func() error { _, err := service.Detail(device.ID); return err },
		func() error { _, _, err := service.DeviceOrganization(device.ID); return err },
		func() error { _, err := service.LocalNetworks(); return err },
		func() error { _, err := service.RefreshDevice(context.Background(), device.ID); return err },
		func() error { return service.Metadata(device.ID, "", "", "") },
	}
	for i, operation := range operations {
		if err := operation(); !errors.Is(err, domain.ErrClosed) {
			t.Fatalf("closed operation %d error=%v", i, err)
		}
	}
}

func TestReadingPersistenceFailureIsReturnedWithoutDeviceFailure(t *testing.T) {
	var readDevice domain.Device
	s, repository, device := workflowFixture(t, workflowClient{read: func(context.Context, string, string, bool) (domain.Reading, error) {
		return deviceReading(readDevice), nil
	}})
	readDevice = device
	failure := errors.New("reading save failed")
	repository.saveErr = failure
	_, err := s.RefreshDevice(t.Context(), device.ID)
	after, _ := repository.Device(device.ID)
	if !errors.Is(err, failure) || errors.Is(err, domain.ErrDeviceRead) || after.LastChecked != device.LastChecked {
		t.Fatalf("persistence failure lost or reported as a device outage: %v", err)
	}
}

func TestConcurrentCaptureRecoversConfigurationWarningBeforeStatusPublication(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var readDevice domain.Device
	service, repository, device := workflowFixture(t, workflowClient{read: func(ctx context.Context, address, mac string, config bool) (domain.Reading, error) {
		reading := deviceReading(readDevice)
		if !config {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return domain.Reading{}, ctx.Err()
			}
			reading.Config, reading.ConfigAttempted = nil, false
		}
		return reading, nil
	}})
	readDevice = device
	device.ConfigAvailable, device.Problem = false, "configuration unavailable"
	repository.devices[device.ID] = device
	completed := make(chan error, 1)
	go func() { _, err := service.RefreshDevice(context.Background(), device.ID); completed <- err }()
	waitForJob(t, started)
	if _, err := service.Capture(context.Background(), device.ID); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	after, _ := repository.Device(device.ID)
	if !after.ConfigAvailable || after.Problem != "" {
		t.Fatalf("status refresh restored an obsolete configuration warning: %+v", after)
	}
}

func TestRefreshDeviceRepositoryLookupFailuresRemainDistinctFromReadFailures(t *testing.T) {
	for _, afterRead := range []bool{false, true} {
		t.Run(fmt.Sprintf("after-read=%t", afterRead), func(t *testing.T) {
			failure := errors.New("device lookup failed")
			var readDevice domain.Device
			var repository *memoryRepository
			var reads int
			service, saved, device := workflowFixture(t, workflowClient{read: func(context.Context, string, string, bool) (domain.Reading, error) {
				reads++
				if afterRead {
					repository.deviceErr = failure
				}
				return deviceReading(readDevice), nil
			}})
			readDevice, repository = device, saved
			if !afterRead {
				repository.deviceErr = failure
			}
			_, err := service.RefreshDevice(context.Background(), device.ID)
			repository.deviceErr = nil
			after, _ := repository.Device(device.ID)
			if !errors.Is(err, failure) || errors.Is(err, domain.ErrDeviceRead) || after.LastChecked != device.LastChecked {
				t.Fatalf("repository error was classified as a device read failure: %v", err)
			}
			if !afterRead && reads != 0 {
				t.Fatalf("lookup stage changed network reads: %d", reads)
			}
		})
	}
}
