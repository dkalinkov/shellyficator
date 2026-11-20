package web

import (
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

func TestCaptureAndBaselineApprovalUseFreshSettings(t *testing.T) {
	var reads atomic.Int32
	var failConfig atomic.Bool
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("device write attempted: %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{"switch:0":{"output":true}}`))
		case "/rpc/Shelly.GetConfig":
			reads.Add(1)
			if failConfig.Load() {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			mode := "detached"
			if reads.Load() == 1 {
				mode = "follow"
			}
			_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"` + mode + `"}}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	})
	if err := s.store.Metadata(id, "Bedroom", "lighting", "Keep this note"); err != nil {
		t.Fatal(err)
	}
	before, err := s.store.Device(id)
	if err != nil {
		t.Fatal(err)
	}
	capture, _ := detailEvent(s, id, http.MethodPost, "/capture", nil)
	if err := s.capture(capture); err != nil {
		t.Fatal(err)
	}
	captured, err := s.store.Device(id)
	if err != nil || captured.BaselineID != before.BaselineID || captured.Config["switch:0"].(map[string]any)["in_mode"] != "follow" {
		t.Fatal("capture did not save fresh settings or replaced the approved baseline")
	}

	event, response := detailEvent(s, id, http.MethodPost, "/baseline", nil)
	if err := s.baseline(event); err != nil {
		t.Fatal(err)
	}
	approved, err := s.store.Device(id)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := s.store.Baseline(approved)
	if err != nil || approved.BaselineID == before.BaselineID || baseline["switch:0"].(map[string]any)["in_mode"] != "detached" || !strings.Contains(response.Header().Get("Location"), "Baseline+updated.") {
		t.Fatal("approval did not select a fresh capture")
	}

	failConfig.Store(true)
	event, response = detailEvent(s, id, http.MethodPost, "/baseline", nil)
	if err := s.baseline(event); err != nil || !strings.Contains(response.Header().Get("Location"), "Baseline+unchanged") {
		t.Fatalf("failed read appeared to approve the baseline: %v", err)
	}

	failConfig.Store(false)
	s.store.App.OnRecordCreate("snapshots").BindFunc(func(*core.RecordEvent) error { return errors.New("capture could not be saved") })
	event, _ = detailEvent(s, id, http.MethodPost, "/baseline", nil)
	var apiError *router.ApiError
	if err := s.baseline(event); !errors.As(err, &apiError) || apiError.Status != http.StatusBadRequest {
		t.Fatalf("failed save was not reported separately: %v", err)
	}
	after, err := s.store.Device(id)
	if err != nil || after.BaselineID != approved.BaselineID || after.ConfigAvailable || after.Notes != "Keep this note" || after.Room != "Bedroom" {
		t.Fatalf("failed approval persisted changes: %+v err=%v", after, err)
	}
	if snapshots, err := s.store.Snapshots(id); err != nil || len(snapshots) != 3 {
		t.Fatalf("expected original, manual, and approved captures: %v %v", snapshots, err)
	}
}
