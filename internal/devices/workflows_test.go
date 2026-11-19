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

// These fixtures intentionally implement the service contracts without a
// database or protocol adapter, so the application workflows stand on their own.
type memoryRepository struct {
	mu                        sync.Mutex
	devices                   map[string]domain.Device
	snapshots                 map[string][]domain.Snapshot
	baselines                 map[string]map[string]any
	notes                     map[string]map[string]string
	devicesErr, deviceErr     error
	baselineErr, notesErr     error
	snapshotsErr, optionsErr  error
	saveErr, failedErr        error
	saves, captures, approves int
	failures                  int
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		devices: map[string]domain.Device{}, snapshots: map[string][]domain.Snapshot{},
		baselines: map[string]map[string]any{}, notes: map[string]map[string]string{},
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
	return r.save(reading, capture, false)
}

func (r *memoryRepository) ApproveReading(reading domain.Reading) (domain.Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.save(reading, true, true)
}

func (r *memoryRepository) save(reading domain.Reading, capture, approve bool) (domain.Device, error) {
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
	r.saves++
	if capture && reading.Config != nil {
		r.captures++
		snapshot := domain.Snapshot{ID: fmt.Sprintf("snapshot-%d", r.captures), DeviceID: device.ID, Config: reading.Config}
		r.snapshots[device.ID] = append(r.snapshots[device.ID], snapshot)
		if approve {
			r.approves++
			device.BaselineID = snapshot.ID
			r.baselines[device.ID] = reading.Config
		}
	}
	r.devices[device.ID] = device
	return device, nil
}

func (r *memoryRepository) Failed(id, problem string, mismatch bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failedErr != nil {
		return r.failedErr
	}
	r.failures++
	r.devices[id] = r.devices[id].FailedCheck(problem, mismatch, time.Now())
	return nil
}

func (r *memoryRepository) Baseline(device domain.Device) (map[string]any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.baselines[device.ID], r.baselineErr
}

func (r *memoryRepository) ChannelNotes(id string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.notes[id], r.notesErr
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

func (r *memoryRepository) Metadata(id, room, tags, notes string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	device := r.devices[id]
	device.Room, device.Tags, device.Notes = room, domain.SplitTags(tags), notes
	r.devices[id] = device
	return nil
}

func (r *memoryRepository) Organization(id, room, tags string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	device := r.devices[id]
	device.Room, device.Tags = room, domain.SplitTags(tags)
	r.devices[id] = device
	return nil
}

func (r *memoryRepository) SaveChannelNote(id, component, notes string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.notes[id] == nil {
		r.notes[id] = map[string]string{}
	}
	r.notes[id][component] = notes
	return nil
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
		{"detail baseline", func(r *memoryRepository) { r.baselineErr = failure }, true},
		{"detail notes", func(r *memoryRepository) { r.notesErr = failure }, true},
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

func TestCaptureAndApprovalUseFreshReadsAndSeparateHistoryOperations(t *testing.T) {
	var readDevice domain.Device
	var reads int
	client := workflowClient{read: func(ctx context.Context, address, mac string, config bool) (domain.Reading, error) {
		if !config || address != readDevice.Address || mac != readDevice.MAC {
			t.Fatalf("fresh configuration read lost address, identity or configuration mode")
		}
		reads++
		return deviceReading(readDevice), nil
	}}
	service, repository, device := workflowFixture(t, client)
	readDevice = device
	captured, err := service.Capture(context.Background(), device.ID)
	if err != nil || captured.BaselineID != device.BaselineID || repository.captures != 1 || repository.approves != 0 {
		t.Fatalf("capture changed baseline or failed to snapshot: device=%+v err=%v", captured, err)
	}
	approved, err := service.ApproveBaseline(context.Background(), device.ID)
	if err != nil || reads != 2 || repository.captures != 2 || repository.approves != 1 || approved.BaselineID != "snapshot-2" {
		t.Fatalf("approval did not atomically select a fresh capture: device=%+v err=%v", approved, err)
	}
	if approved.Notes != device.Notes || approved.Room != device.Room || len(approved.Tags) != 1 || approved.Tags[0] != "lamp" {
		t.Fatal("fresh configuration workflow lost local metadata")
	}
}

func TestMissingConfigurationPreservesSavedConfigurationAndBaseline(t *testing.T) {
	for _, approve := range []bool{false, true} {
		t.Run(fmt.Sprintf("approve=%t", approve), func(t *testing.T) {
			var readDevice domain.Device
			service, repository, device := workflowFixture(t, workflowClient{read: func(context.Context, string, string, bool) (domain.Reading, error) {
				reading := deviceReading(readDevice)
				reading.Config, reading.Problem = nil, "settings unavailable"
				return reading, nil
			}})
			readDevice = device
			var err error
			if approve {
				_, err = service.ApproveBaseline(context.Background(), device.ID)
			} else {
				_, err = service.Capture(context.Background(), device.ID)
			}
			after, _ := repository.Device(device.ID)
			if !errors.Is(err, domain.ErrConfigUnavailable) || repository.saves != 1 || repository.captures != 0 || repository.approves != 0 || repository.failures != 0 {
				t.Fatalf("missing configuration created history or lost valid observations: %v", err)
			}
			if after.ConfigAvailable || !after.StatusAvailable || !after.Reachable || after.BaselineID != device.BaselineID || after.LastCaptured != device.LastCaptured || after.Config["switch:0"].(map[string]any)["in_mode"] != "momentary" || after.Status["switch:0"].(map[string]any)["output"] != true || after.Problem != "settings unavailable" {
				t.Fatal("missing configuration changed baseline/cached config or discarded fresh status")
			}
		})
	}
}

func TestStatusRefreshPreservesConfigurationWarningAndReportsAvailability(t *testing.T) {
	for _, available := range []bool{false, true} {
		t.Run(fmt.Sprintf("status=%t", available), func(t *testing.T) {
			var readDevice domain.Device
			service, repository, device := workflowFixture(t, workflowClient{read: func(ctx context.Context, address, mac string, config bool) (domain.Reading, error) {
				if config {
					t.Fatal("status refresh requested configuration")
				}
				reading := deviceReading(readDevice)
				reading.Config, reading.ConfigAttempted = nil, false
				if !available {
					reading.Status = nil
				}
				return reading, nil
			}})
			readDevice = device
			device.ConfigAvailable = false
			repository.devices[device.ID] = device
			result, err := service.RefreshDevice(context.Background(), device.ID)
			after, _ := repository.Device(device.ID)
			if err != nil || result.StatusAvailable != available || after.ConfigAvailable || after.Problem != "Configuration is unavailable. Capture config to retry." || after.BaselineID != device.BaselineID || repository.captures != 0 {
				t.Fatalf("status refresh lost configuration warning or changed history: result=%+v device=%+v err=%v", result, after, err)
			}
		})
	}
}

func TestSingleDeviceIdentityFailureAndPersistenceErrorPropagation(t *testing.T) {
	storageFailure := errors.New("cannot persist failure")
	for _, workflow := range []string{"status", "capture", "approve"} {
		t.Run(workflow, func(t *testing.T) {
			service, repository, device := workflowFixture(t, workflowClient{read: func(context.Context, string, string, bool) (domain.Reading, error) {
				return domain.Reading{}, domain.ErrIdentity
			}})
			run := func() error {
				switch workflow {
				case "status":
					_, err := service.RefreshDevice(context.Background(), device.ID)
					return err
				case "capture":
					_, err := service.Capture(context.Background(), device.ID)
					return err
				default:
					_, err := service.ApproveBaseline(context.Background(), device.ID)
					return err
				}
			}
			if err := run(); !errors.Is(err, domain.ErrIdentity) || !errors.Is(err, domain.ErrDeviceRead) {
				t.Fatalf("identity failure lost: %v", err)
			}
			after, _ := repository.Device(device.ID)
			if !after.IdentityMismatch || after.Reachable || after.MAC != device.MAC || repository.saves != 0 || repository.failures != 1 {
				t.Fatal("identity failure rebound device or failed to mark health immediately")
			}
			repository.failedErr = storageFailure
			if err := run(); !errors.Is(err, storageFailure) || errors.Is(err, domain.ErrDeviceRead) {
				t.Fatalf("failure persistence error hidden: %v", err)
			}
		})
	}
}

func TestSingleDeviceCancellationKeepsCachedInventoryAvailableAndDrainsWork(t *testing.T) {
	for _, workflow := range []string{"status", "capture", "approve"} {
		for _, callerCancels := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/caller=%t", workflow, callerCancels), func(t *testing.T) {
				started, drained := make(chan struct{}), make(chan struct{})
				service, repository, device := workflowFixture(t, workflowClient{read: func(ctx context.Context, address, mac string, config bool) (domain.Reading, error) {
					close(started)
					<-ctx.Done()
					close(drained)
					return domain.Reading{}, ctx.Err()
				}})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				completed := make(chan error, 1)
				go func() {
					var err error
					switch workflow {
					case "status":
						_, err = service.RefreshDevice(ctx, device.ID)
					case "capture":
						_, err = service.Capture(ctx, device.ID)
					case "approve":
						_, err = service.ApproveBaseline(ctx, device.ID)
					}
					completed <- err
				}()
				waitForJob(t, started)
				inventoryDone := make(chan error, 1)
				go func() { _, err := service.Inventory(); inventoryDone <- err }()
				select {
				case err := <-inventoryDone:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(time.Second):
					t.Fatal("inventory query waited for a single-device network read")
				}
				if callerCancels {
					cancel()
				} else {
					closeService(t, service)
				}
				waitForJob(t, drained)
				select {
				case err := <-completed:
					want := domain.ErrClosed
					if callerCancels {
						want = context.Canceled
					}
					if !errors.Is(err, want) || errors.Is(err, domain.ErrDeviceRead) {
						t.Fatalf("cancellation error=%v want=%v", err, want)
					}
				case <-time.After(time.Second):
					t.Fatal("single-device operation did not drain")
				}
				after, _ := repository.Device(device.ID)
				if repository.saves != 0 || repository.failures != 0 || after.LastChecked != device.LastChecked || after.BaselineID != device.BaselineID {
					t.Fatal("cancellation changed health or history")
				}
			})
		}
	}
}

func TestApprovalSaveFailureIdentifiesStageAndPreservesOriginalCause(t *testing.T) {
	var readDevice domain.Device
	service, repository, device := workflowFixture(t, workflowClient{read: func(context.Context, string, string, bool) (domain.Reading, error) {
		return deviceReading(readDevice), nil
	}})
	readDevice = device
	failure := errors.New("transaction failed")
	repository.saveErr = failure
	_, err := service.ApproveBaseline(context.Background(), device.ID)
	if !errors.Is(err, domain.ErrBaselineSave) || !errors.Is(err, failure) || errors.Is(err, domain.ErrDeviceRead) {
		t.Fatalf("approval failure lost stage or cause: %v", err)
	}
	if repository.approves != 0 || repository.captures != 0 || repository.devices[device.ID].BaselineID != device.BaselineID {
		t.Fatal("failed approval changed snapshot history or baseline")
	}
}

func TestClosedServiceRejectsQueriesAndSynchronousWork(t *testing.T) {
	service, _, device := workflowFixture(t, workflowClient{})
	service.Close()
	operations := []func() error{
		func() error { _, err := service.Inventory(); return err },
		func() error { _, err := service.Detail(device.ID); return err },
		func() error { _, err := service.OrganizationOptions(); return err },
		func() error { _, err := service.LocalNetworks(); return err },
		func() error { _, err := service.RefreshDevice(context.Background(), device.ID); return err },
		func() error { _, err := service.Capture(context.Background(), device.ID); return err },
		func() error { _, err := service.ApproveBaseline(context.Background(), device.ID); return err },
		func() error { return service.Metadata(device.ID, "", "", "") },
		func() error { return service.Organization(device.ID, "", "") },
		func() error { return service.SaveChannelNote(device.ID, "switch:0", "") },
		func() error { return service.Discover("", true) },
	}
	for i, operation := range operations {
		if err := operation(); !errors.Is(err, domain.ErrClosed) {
			t.Fatalf("closed operation %d error=%v", i, err)
		}
	}
}

func TestReadingPersistenceFailureIsReturnedWithoutDeviceFailure(t *testing.T) {
	for _, capture := range []bool{false, true} {
		t.Run(fmt.Sprintf("capture=%t", capture), func(t *testing.T) {
			var readDevice domain.Device
			service, repository, device := workflowFixture(t, workflowClient{read: func(context.Context, string, string, bool) (domain.Reading, error) {
				return deviceReading(readDevice), nil
			}})
			readDevice = device
			failure := errors.New("reading save failed")
			repository.saveErr = failure
			var err error
			if capture {
				_, err = service.Capture(context.Background(), device.ID)
			} else {
				_, err = service.RefreshDevice(context.Background(), device.ID)
			}
			if !errors.Is(err, failure) || errors.Is(err, domain.ErrDeviceRead) || repository.failures != 0 {
				t.Fatalf("persistence failure lost or reported as device outage: %v", err)
			}
		})
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
	if !after.ConfigAvailable || after.Problem != "" || repository.captures != 1 {
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
			if !errors.Is(err, failure) || errors.Is(err, domain.ErrDeviceRead) || repository.failures != 0 || repository.saves != 0 {
				t.Fatalf("repository error was classified as a device read failure: %v", err)
			}
			if !afterRead && reads != 0 || afterRead && reads != 1 {
				t.Fatalf("lookup stage changed network reads: %d", reads)
			}
		})
	}
}

func (r *memoryRepository) DiscoveryNetworks() ([]string, error) { return nil, nil }
func (r *memoryRepository) RememberNetwork(string) error         { return nil }
func (r *memoryRepository) ForgetNetwork(string) error           { return nil }
