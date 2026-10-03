package devices

import (
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigurationRefreshAndRecoveryPreserveHistory(t *testing.T) {
	var configs atomic.Int32
	var failConfig atomic.Bool
	s, id := refreshTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{"input:0":{"state":null},"switch:0":{"output":true}}`))
		case "/rpc/Shelly.GetConfig":
			configs.Add(1)
			if failConfig.Load() {
				http.Error(w, "settings unavailable", http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{"input:0":{"type":"button","enable":true},"switch:0":{"in_mode":"detached"}}`))
		default:
			t.Errorf("unexpected device request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	record, _ := s.store.Device(id)
	baselineID := record.BaselineID
	assertHistory := func() {
		t.Helper()
		record, err := s.store.Device(id)
		if err != nil {
			t.Fatal(err)
		}
		snapshots, err := s.store.Snapshots(id)
		if err != nil || len(snapshots) != 1 || snapshots[0].ID != baselineID || record.BaselineID != baselineID {
			t.Fatalf("refresh changed capture history or baseline: snapshots=%v err=%v", snapshots, err)
		}
		baseline, err := s.store.Baseline(record)
		if err != nil || baseline["switch:0"].(map[string]any)["in_mode"] != "momentary" {
			t.Fatal("refresh modified approved baseline contents")
		}
	}

	s.refreshDevices(false)
	if configs.Load() != 0 {
		t.Fatal("status poll reread recent settings")
	}
	record, _ = s.store.Device(id)
	setDeviceField(t, s, id, "last_captured", time.Now().UTC().Add(-6*time.Minute))
	s.refreshDevices(false)
	record, _ = s.store.Device(id)
	if configs.Load() != 1 || !record.ConfigCurrent() || record.Config["switch:0"].(map[string]any)["in_mode"] != "detached" || time.Since(record.LastCaptured) > time.Minute {
		t.Fatal("periodic configuration read did not update settings")
	}
	assertHistory()

	lastRead := record.LastCaptured
	failConfig.Store(true)
	s.refreshDevices(true)
	record, _ = s.store.Device(id)
	if !record.Reachable || !record.StatusAvailable || record.ConfigAvailable || record.LastCaptured != lastRead || record.Config["switch:0"].(map[string]any)["in_mode"] != "detached" || record.Status["switch:0"].(map[string]any)["output"] != true || record.Status["input:0"].(map[string]any)["state"] != nil {
		t.Fatal("failed settings read lost saved settings or live status")
	}
	assertHistory()

	failConfig.Store(false)
	s.refreshDevices(false)
	record, _ = s.store.Device(id)
	if configs.Load() != 4 || !record.ConfigAvailable || record.Problem != "" {
		t.Fatal("next poll did not retry unavailable configuration and clear warning")
	}
	if err := s.store.Failed(id, "offline", false); err != nil {
		t.Fatal(err)
	}
	s.refreshDevices(false)
	record, _ = s.store.Device(id)
	if configs.Load() != 5 || !record.Reachable || !record.ConfigAvailable {
		t.Fatal("reconnected device did not reread configuration")
	}
	assertHistory()
}

func TestManualRefreshDuringStatusPollQueuesConfigurationRead(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var statuses, configs atomic.Int32
	s, _ := refreshTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			if statuses.Add(1) == 1 {
				close(started)
				<-release
			}
			_, _ = w.Write([]byte(`{"switch:0":{"output":true}}`))
		case "/rpc/Shelly.GetConfig":
			configs.Add(1)
			_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"detached"}}`))
		}
	})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	done := s.Refresh(false)
	waitForJob(t, started)
	if s.Refresh(true) != done || s.Refresh(true) != done {
		t.Fatal("concurrent requests did not share refresh completion")
	}
	close(release)
	waitForJob(t, done)
	if statuses.Load() != 2 || configs.Load() != 1 {
		t.Fatalf("manual configuration pass dropped or duplicated: status=%d config=%d", statuses.Load(), configs.Load())
	}
	if s.Refresh(false) != nil {
		t.Fatal("shared refresh throttle did not suppress immediate polling")
	}
}

func TestRefreshMissesKeepInventoryValuesAndRecoverWithoutChangingHistory(t *testing.T) {
	var fail, mismatch atomic.Bool
	var identityRequests atomic.Int32
	s, id := refreshTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			identityRequests.Add(1)
			if fail.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if mismatch.Load() {
				_, _ = w.Write([]byte(`{"id":"other-device","mac":"112233445566","gen":4}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{"input:0":{"state":false},"switch:0":{"output":true},"wifi":{"rssi":-55},"cloud":{"connected":true}}`))
		case "/rpc/Shelly.GetConfig":
			_, _ = w.Write([]byte(`{"input:0":{"type":"button","enable":true},"switch:0":{"in_mode":"momentary"}}`))
		}
	})
	s.refreshDevices(true)
	before, _ := s.store.Device(id)
	baselineID := before.BaselineID
	fail.Store(true)
	s.refreshDevices(false)
	after, _ := s.store.Device(id)
	status := after.Status
	if after.ConsecutiveFailures != 1 || identityRequests.Load() != 3 || !after.Reachable || after.StatusCurrent() || after.ConfigCurrent() || status["cloud"].(map[string]any)["connected"] != true || status["switch:0"].(map[string]any)["output"] != true || status["input:0"].(map[string]any)["state"] != false {
		t.Fatal("one missed check lost saved values, changed reachability, or counted the retry twice")
	}
	setDeviceField(t, s, id, "last_seen", time.Now().UTC().Add(-2*time.Minute))
	s.refreshDevices(false)
	s.refreshDevices(false)
	after, _ = s.store.Device(id)
	if after.Reachable || after.ConsecutiveFailures != 3 {
		t.Fatal("sustained outage did not become offline")
	}
	fail.Store(false)
	s.refreshDevices(false)
	after, _ = s.store.Device(id)
	snapshots, err := s.store.Snapshots(id)
	if err != nil || len(snapshots) != 1 || snapshots[0].ID != baselineID || after.BaselineID != baselineID || !after.StatusCurrent() || !after.ConfigCurrent() || after.ConsecutiveFailures != 0 {
		t.Fatal("recovery did not restore health or changed history")
	}
	mismatch.Store(true)
	count := identityRequests.Load()
	s.refreshDevices(false)
	after, _ = s.store.Device(id)
	if !after.IdentityMismatch || after.Reachable || identityRequests.Load() != count+1 || after.MAC != "AABBCCDDEEFF" {
		t.Fatal("identity mismatch was delayed, retried, or rebound the saved device")
	}
}
