package devices

import (
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"shellyficator/internal/inventory"
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
	baselineID := record.GetString("baseline")
	assertHistory := func() {
		t.Helper()
		record, err := s.store.Device(id)
		if err != nil {
			t.Fatal(err)
		}
		snapshots, err := s.store.Snapshots(id)
		if err != nil || len(snapshots) != 1 || snapshots[0].Id != baselineID || record.GetString("baseline") != baselineID {
			t.Fatalf("refresh changed capture history or baseline: snapshots=%v err=%v", snapshots, err)
		}
		if s.store.Baseline(record)["switch:0"].(map[string]any)["in_mode"] != "momentary" {
			t.Fatal("refresh modified approved baseline contents")
		}
	}

	s.refreshDevices(false)
	if configs.Load() != 0 {
		t.Fatal("status poll reread recent settings")
	}
	record, _ = s.store.Device(id)
	record.Set("last_captured", time.Now().UTC().Add(-6*time.Minute))
	if err := s.store.App.Save(record); err != nil {
		t.Fatal(err)
	}
	s.refreshDevices(false)
	record, _ = s.store.Device(id)
	if configs.Load() != 1 || !inventory.ConfigCurrent(record) || inventory.Object(record, "config")["switch:0"].(map[string]any)["in_mode"] != "detached" || time.Since(record.GetDateTime("last_captured").Time()) > time.Minute {
		t.Fatal("periodic configuration read did not update settings")
	}
	assertHistory()

	lastRead := record.GetString("last_captured")
	failConfig.Store(true)
	s.refreshDevices(true)
	record, _ = s.store.Device(id)
	if !record.GetBool("reachable") || !record.GetBool("status_available") || record.GetBool("config_available") || record.GetString("last_captured") != lastRead || inventory.Object(record, "config")["switch:0"].(map[string]any)["in_mode"] != "detached" || inventory.Object(record, "status")["switch:0"].(map[string]any)["output"] != true || inventory.Object(record, "status")["input:0"].(map[string]any)["state"] != nil {
		t.Fatal("failed settings read lost saved settings or live status")
	}
	assertHistory()

	failConfig.Store(false)
	s.refreshDevices(false)
	record, _ = s.store.Device(id)
	if configs.Load() != 4 || !record.GetBool("config_available") || record.GetString("problem") != "" {
		t.Fatal("next poll did not retry unavailable configuration and clear warning")
	}
	if err := s.store.Failed(id, "offline", false); err != nil {
		t.Fatal(err)
	}
	s.refreshDevices(false)
	record, _ = s.store.Device(id)
	if configs.Load() != 5 || !record.GetBool("reachable") || !record.GetBool("config_available") {
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
	baselineID := before.GetString("baseline")
	fail.Store(true)
	s.refreshDevices(false)
	after, _ := s.store.Device(id)
	status := inventory.Object(after, "status")
	if after.GetInt("consecutive_failures") != 1 || identityRequests.Load() != 3 || !after.GetBool("reachable") || inventory.StatusCurrent(after) || inventory.ConfigCurrent(after) || status["cloud"].(map[string]any)["connected"] != true || status["switch:0"].(map[string]any)["output"] != true || status["input:0"].(map[string]any)["state"] != false {
		t.Fatal("one missed check lost saved values, changed reachability, or counted the retry twice")
	}
	after.Set("last_seen", time.Now().UTC().Add(-2*time.Minute))
	if err := s.store.App.Save(after); err != nil {
		t.Fatal(err)
	}
	s.refreshDevices(false)
	s.refreshDevices(false)
	after, _ = s.store.Device(id)
	if after.GetBool("reachable") || after.GetInt("consecutive_failures") != 3 {
		t.Fatal("sustained outage did not become offline")
	}
	fail.Store(false)
	s.refreshDevices(false)
	after, _ = s.store.Device(id)
	snapshots, err := s.store.Snapshots(id)
	if err != nil || len(snapshots) != 1 || snapshots[0].Id != baselineID || after.GetString("baseline") != baselineID || !inventory.StatusCurrent(after) || !inventory.ConfigCurrent(after) || after.GetInt("consecutive_failures") != 0 {
		t.Fatal("recovery did not restore health or changed history")
	}
	mismatch.Store(true)
	count := identityRequests.Load()
	s.refreshDevices(false)
	after, _ = s.store.Device(id)
	if !after.GetBool("identity_mismatch") || after.GetBool("reachable") || identityRequests.Load() != count+1 || after.GetString("mac") != "AABBCCDDEEFF" {
		t.Fatal("identity mismatch was delayed, retried, or rebound the saved device")
	}
}
