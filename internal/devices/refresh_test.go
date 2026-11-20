package devices

import (
	"net/http"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigurationRefreshAndRecoveryPreserveHistory(t *testing.T) {
	var configs atomic.Int32
	var failConfig, recovered atomic.Bool
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
			if recovered.Load() {
				_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"follow"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"input:0":{"type":"button","enable":true},"switch:0":{"in_mode":"detached"}}`))
		default:
			t.Errorf("unexpected device request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	initial, err := s.store.Device(id)
	if err != nil {
		t.Fatal(err)
	}

	s.refreshDevices(false)
	if configs.Load() != 0 {
		t.Fatal("status poll reread recent settings")
	}
	setDeviceField(t, s, id, "last_captured", time.Now().UTC().Add(-6*time.Minute))
	s.refreshDevices(false)
	record, _ := s.store.Device(id)
	if configs.Load() != 1 || !record.ConfigCurrent() || record.Config["switch:0"].(map[string]any)["in_mode"] != "detached" || time.Since(record.LastCaptured) > time.Minute {
		t.Fatal("periodic configuration read did not update settings")
	}

	lastRead := record.LastCaptured
	failConfig.Store(true)
	s.refreshDevices(true)
	record, _ = s.store.Device(id)
	if !record.Reachable || !record.StatusAvailable || record.ConfigAvailable || record.LastCaptured != lastRead || record.Config["switch:0"].(map[string]any)["in_mode"] != "detached" || record.Status["switch:0"].(map[string]any)["output"] != true {
		t.Fatal("failed settings read lost saved settings or live status")
	}

	failConfig.Store(false)
	recovered.Store(true)
	s.refreshDevices(false)
	record, _ = s.store.Device(id)
	if !record.ConfigCurrent() || record.Config["switch:0"].(map[string]any)["in_mode"] != "follow" || record.Problem != "" {
		t.Fatal("next poll did not retry unavailable configuration and clear warning")
	}
	snapshots, err := s.store.Snapshots(id)
	if err != nil || len(snapshots) != 1 || snapshots[0].ID != initial.BaselineID || record.BaselineID != initial.BaselineID {
		t.Fatalf("refresh changed capture history or baseline: snapshots=%v err=%v", snapshots, err)
	}
	baseline, err := s.store.Baseline(record)
	if err != nil || !reflect.DeepEqual(baseline, initial.Config) {
		t.Fatal("refresh modified approved baseline contents")
	}
}

func TestManualRefreshDuringStatusPollQueuesConfigurationRead(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var statuses, configs atomic.Int32
	s, id := refreshTestService(t, func(w http.ResponseWriter, r *http.Request) {
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
	firstManual, secondManual := s.Refresh(true), s.Refresh(true)
	close(release)
	for _, request := range []<-chan struct{}{firstManual, secondManual} {
		waitForJob(t, request)
		device, err := s.store.Device(id)
		if err != nil || !device.ConfigCurrent() || device.Config["switch:0"].(map[string]any)["in_mode"] != "detached" {
			t.Fatalf("manual refresh finished before saving fresh configuration: %+v, %v", device, err)
		}
	}
	waitForJob(t, done)
	if configs.Load() != 1 {
		t.Fatalf("concurrent manual requests read settings %d times", configs.Load())
	}
	if s.Refresh(false) != nil {
		t.Fatal("shared refresh throttle did not suppress immediate polling")
	}
}

func TestRefreshCountsRetriesAsOneMissAndRejectsReplacementDevices(t *testing.T) {
	var fail, recovered, mismatch atomic.Bool
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
			if recovered.Load() {
				_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"follow"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"input:0":{"type":"button","enable":true},"switch:0":{"in_mode":"momentary"}}`))
		}
	})
	s.refreshDevices(true)
	before, _ := s.store.Device(id)
	fail.Store(true)
	s.refreshDevices(false)
	after, _ := s.store.Device(id)
	if after.ConsecutiveFailures != 1 || identityRequests.Load() != 3 || !after.Reachable || after.StatusCurrent() || after.ConfigCurrent() || !reflect.DeepEqual(after.Status, before.Status) {
		t.Fatal("one missed check lost saved values, changed reachability, or counted the retry twice")
	}
	fail.Store(false)
	recovered.Store(true)
	s.refreshDevices(false)
	after, _ = s.store.Device(id)
	if !after.StatusCurrent() || !after.ConfigCurrent() || after.Config["switch:0"].(map[string]any)["in_mode"] != "follow" || after.ConsecutiveFailures != 0 {
		t.Fatal("polling did not reread settings and restore health after a missed check")
	}
	mismatch.Store(true)
	count := identityRequests.Load()
	s.refreshDevices(false)
	after, _ = s.store.Device(id)
	if !after.IdentityMismatch || after.Reachable || identityRequests.Load() != count+1 || after.MAC != "AABBCCDDEEFF" {
		t.Fatal("identity mismatch was delayed, retried, or rebound the saved device")
	}
}
