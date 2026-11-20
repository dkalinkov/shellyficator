package devices

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"shellyficator/internal/domain"
)

func updateFixture(t *testing.T, mode string) (*Service, *memoryRepository, []domain.FirmwareSelection, *atomic.Int32) {
	t.Helper()
	repository := newMemoryRepository()
	var writes atomic.Int32
	var updated atomic.Bool
	var finalStep atomic.Bool
	device := domain.Device{ID: "one", DeviceID: "reported-one", MAC: "AABBCCDDEEFF", Address: "http://127.0.0.2:1234", Firmware: "2.1.0", Reachable: true, StatusAvailable: true, ConfigAvailable: true, BaselineID: "approved", Config: map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}}}
	repository.devices[device.ID] = device
	repository.baselines[device.ID] = device.Config
	client := workflowClient{read: func(ctx context.Context, address, mac string, config bool) (domain.Reading, error) {
		version, uptime := "2.1.0", float64(1000)
		if updated.Load() {
			uptime = 1
			if mode != "unchanged" {
				version = "2.2.0"
			}
		}
		reading := domain.Reading{Identity: domain.Identity{ID: device.DeviceID, MAC: mac, Version: version}, Address: address,
			Status:        map[string]any{"sys": map[string]any{"uptime": uptime}},
			FirmwareCheck: &domain.FirmwareCheck{Firmware: version, CheckedAt: time.Now(), Available: map[string]any{"stable": map[string]any{"version": "2.2.0"}}}}
		if mode == "offer changed" {
			reading.FirmwareCheck.Available["stable"] = map[string]any{"version": "2.3.0"}
		}
		if mode == "staged" && updated.Load() && !finalStep.Load() {
			reading.Identity.Version = "2.1.0"
			reading.Status["sys"].(map[string]any)["available_updates"] = map[string]any{"stable": map[string]any{"version": "2.2.0", "nsteps": float64(1)}}
		}
		if config {
			reading.ConfigAttempted = true
			if mode != "capture failed" && !(mode == "after capture failed" && updated.Load()) {
				reading.Config = device.Config
			}
		}
		return reading, nil
	}, update: func(ctx context.Context, address, mac, stage string) error {
		writes.Add(1)
		if stage != "stable" {
			t.Errorf("unexpected stage %s", stage)
		}
		if mode == "rejected" {
			return domain.ErrFirmwareRejected
		}
		updated.Store(true)
		if mode == "staged" {
			time.AfterFunc(3*time.Minute, func() { finalStep.Store(true) })
		}
		if mode == "lost response" {
			return errors.New("connection closed after dispatch")
		}
		return nil
	}}
	s := New(repository, client, Options{})
	t.Cleanup(s.Close)
	return s, repository, []domain.FirmwareSelection{{DeviceID: "one", Stage: "stable", FromVersion: "2.1.0", TargetVersion: "2.2.0"}}, &writes
}

func waitForFirmwareBatch(t *testing.T, s *Service) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Minute)
	for s.FirmwareRunning() {
		if time.Now().After(deadline) {
			t.Fatal("firmware batch did not finish")
		}
		time.Sleep(5 * time.Second)
	}
}

func TestFirmwareWorkflowVerifiesActualVersionAndPreservesBaseline(t *testing.T) {
	for _, test := range []struct {
		mode, phase      string
		writes, captures int32
	}{
		{"success", "succeeded", 1, 2}, {"lost response", "succeeded", 1, 2},
		{"staged", "succeeded", 1, 2},
		{"unchanged", "failed", 1, 1}, {"rejected", "failed", 1, 1},
		{"offer changed", "failed", 0, 0}, {"capture failed", "failed", 0, 0},
		{"after capture failed", "warning", 1, 1},
	} {
		t.Run(test.mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, repo, selections, writes := updateFixture(t, test.mode)
				caller, cancel := context.WithCancel(context.Background())
				if err := s.StartFirmwareUpdates(caller, selections); err != nil {
					t.Fatal(err)
				}
				cancel() // A disconnected browser cannot cancel an accepted installation.
				waitForFirmwareBatch(t, s)
				device, err := s.Device("one")
				if err != nil || device.FirmwareUpdate == nil || device.FirmwareUpdate.Phase != test.phase || writes.Load() != test.writes {
					t.Fatalf("incorrect result: %+v writes=%d", device.FirmwareUpdate, writes.Load())
				}
				snapshots, _ := repo.Snapshots(device.ID)
				if len(snapshots) != int(test.captures) || device.BaselineID != "approved" {
					t.Fatalf("incorrect history: captures=%d baseline=%s", len(snapshots), device.BaselineID)
				}
				if test.phase == "succeeded" && device.Firmware != "2.2.0" {
					t.Fatal("reported success without the installed target version")
				}
				if test.mode == "unchanged" && device.Firmware != "2.1.0" {
					t.Fatal("failed installation rewrote the installed firmware")
				}
			})
		})
	}
}

func TestFirmwareAdmissionRejectsInvalidAndDuplicateSelections(t *testing.T) {
	for _, mode := range []string{"empty", "duplicate", "stage", "same version", "installed changed", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			s, _, selections, writes := updateFixture(t, "success")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "empty":
				selections = nil
			case "duplicate":
				selections = append(selections, selections[0])
			case "stage":
				selections[0].Stage = "custom"
			case "same version":
				selections[0].TargetVersion = selections[0].FromVersion
			case "installed changed":
				selections[0].FromVersion = "1.0.0"
			case "cancelled":
				cancel()
			}
			if err := s.StartFirmwareUpdates(ctx, selections); err == nil {
				t.Fatal("invalid batch accepted")
			}
			if writes.Load() != 0 {
				t.Fatal("invalid batch sent an update")
			}
		})
	}
}

func TestFirmwareQueueRunsSeriallyAndBlocksDuplicateAdmission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, repo, selections, writes := updateFixture(t, "success")
		firstStarted, release := make(chan struct{}), make(chan struct{})
		client := s.client.(workflowClient)
		original := client.update
		originalRead := client.read
		var secondUpdated atomic.Bool
		client.read = func(ctx context.Context, address, mac string, config bool) (domain.Reading, error) {
			reading, err := originalRead(ctx, address, mac, config)
			if mac == "AABBCCDDEE01" && !secondUpdated.Load() {
				reading.Identity.Version = "2.1.0"
				reading.FirmwareCheck.Firmware = "2.1.0"
				reading.Status["sys"].(map[string]any)["uptime"] = float64(1000)
			}
			return reading, err
		}
		client.update = func(ctx context.Context, address, mac, stage string) error {
			if mac == "AABBCCDDEEFF" {
				close(firstStarted)
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			err := original(ctx, address, mac, stage)
			if mac == "AABBCCDDEE01" {
				secondUpdated.Store(true)
			}
			return err
		}
		s.client = client
		repo.mu.Lock()
		two := repo.devices["one"]
		two.ID, two.DeviceID, two.MAC = "two", "reported-two", "AABBCCDDEE01"
		repo.devices["two"] = two
		repo.mu.Unlock()
		second := selections[0]
		second.DeviceID = "two"
		selections = append(selections, second)
		if err := s.StartFirmwareUpdates(context.Background(), selections); err != nil {
			t.Fatal(err)
		}
		<-firstStarted
		synctest.Wait()
		if err := s.StartFirmwareUpdates(context.Background(), selections); !errors.Is(err, domain.ErrFirmwareBusy) {
			t.Fatalf("concurrent batch accepted: %v", err)
		}
		if writes.Load() != 0 {
			t.Fatal("second update ran while the first was blocked")
		}
		close(release)
		waitForFirmwareBatch(t, s)
		if writes.Load() != 2 {
			t.Fatalf("serial batch did not update both devices: writes=%d", writes.Load())
		}
	})
}

func TestFirmwareAdmissionDoesNotDispatchAfterStorageFailure(t *testing.T) {
	s, repo, selections, writes := updateFixture(t, "success")
	repo.saveErr = errors.New("cannot persist batch")
	if err := s.StartFirmwareUpdates(context.Background(), selections); err == nil {
		t.Fatal("unpersisted batch accepted")
	}
	if writes.Load() != 0 {
		t.Fatal("unpersisted batch sent an update")
	}
}

type interruptedFirmwareStore struct {
	*memoryRepository
	phase   string
	blocked bool
}

func (r *interruptedFirmwareStore) SaveFirmwareUpdates(updates map[string]domain.FirmwareUpdate) error {
	for _, update := range updates {
		if r.blocked && (update.Phase == r.phase || !update.Active()) {
			return errors.New("firmware progress storage unavailable")
		}
	}
	return r.memoryRepository.SaveFirmwareUpdates(updates)
}

func TestFirmwareResultRecoversAfterProgressStorageReturns(t *testing.T) {
	for _, test := range []struct {
		phase, result string
		writes        int32
	}{
		{"capturing", "failed", 0},
		{"starting", "failed", 0},
		{"succeeded", "succeeded", 1},
	} {
		t.Run(test.phase, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, repo, selections, writes := updateFixture(t, "success")
				store := &interruptedFirmwareStore{memoryRepository: repo, phase: test.phase, blocked: true}
				s.store = store
				if err := s.StartFirmwareUpdates(context.Background(), selections); err != nil {
					t.Fatal(err)
				}
				waitForFirmwareBatch(t, s)
				store.blocked = false
				details, err := s.Detail(selections[0].DeviceID)
				if err != nil || details.Device.FirmwareUpdate == nil || details.Device.FirmwareUpdate.Phase != test.result {
					t.Fatalf("terminal result did not recover: update=%+v err=%v", details.Device.FirmwareUpdate, err)
				}
				saved, err := repo.Device(selections[0].DeviceID)
				if err != nil || saved.FirmwareUpdate == nil || saved.FirmwareUpdate.Phase != test.result || writes.Load() != test.writes {
					t.Fatalf("recovery lost the result or repeated the device command: update=%+v writes=%d err=%v", saved.FirmwareUpdate, writes.Load(), err)
				}
			})
		})
	}
}

func TestFirmwareShutdownAndRecoveryNeverRepeatTheCommand(t *testing.T) {
	s, repo, selections, writes := updateFixture(t, "success")
	started := make(chan struct{})
	client := s.client.(workflowClient)
	client.update = func(ctx context.Context, address, mac, stage string) error {
		writes.Add(1)
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	s.client = client
	if err := s.StartFirmwareUpdates(context.Background(), selections); err != nil {
		t.Fatal(err)
	}
	<-started
	s.Close()
	device, err := repo.Device("one")
	if err != nil || device.FirmwareUpdate == nil || device.FirmwareUpdate.Phase != "unconfirmed" || writes.Load() != 1 {
		t.Fatalf("shutdown lost uncertainty: %+v writes=%d", device.FirmwareUpdate, writes.Load())
	}
	for phase, want := range map[string]string{"queued": "failed", "verifying": "unconfirmed"} {
		repo.mu.Lock()
		u := *device.FirmwareUpdate
		u.ID = phase
		u.Phase = phase
		device.FirmwareUpdate = &u
		repo.devices[device.ID] = device
		repo.mu.Unlock()
		restarted := New(repo, client, Options{})
		recovered, err := restarted.Device(device.ID)
		if err != nil || recovered.FirmwareUpdate == nil || recovered.FirmwareUpdate.Phase != want {
			t.Fatalf("restart did not recover %s as %s: %+v %v", phase, want, recovered.FirmwareUpdate, err)
		}
		restarted.Close()
	}
	if writes.Load() != 1 {
		t.Fatal("restart repeated an update command")
	}
}
