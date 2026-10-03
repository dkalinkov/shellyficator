package web

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"shellyficator/internal/domain"
)

func TestBaselineApprovalRequiresFreshConfigAndPreservesFailedApproval(t *testing.T) {
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
			if failConfig.Load() {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"detached"}}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	})
	if err := s.store.Metadata(id, "Bedroom", "lighting", "Keep this note"); err != nil {
		t.Fatal(err)
	}
	event, response := detailEvent(s, id, http.MethodPost, "baseline", nil)
	if err := s.baseline(event); err != nil {
		t.Fatal(err)
	}
	record, err := s.record(id)
	if err != nil {
		t.Fatal(err)
	}
	baselineID := record.GetString("baseline")
	snapshots, err := s.store.Snapshots(id)
	if err != nil || len(snapshots) != 2 || len(domain.Differences(s.baselineConfig(record), map[string]any{"switch:0": map[string]any{"in_mode": "detached"}})) != 0 || !strings.Contains(response.Header().Get("Location"), "Baseline+updated.") {
		t.Fatalf("approval did not capture its fresh settings: history=%v err=%v", snapshots, err)
	}
	failConfig.Store(true)
	event, response = detailEvent(s, id, http.MethodPost, "baseline", nil)
	if err := s.baseline(event); err != nil {
		t.Fatal(err)
	}
	record, err = s.record(id)
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err = s.store.Snapshots(id)
	if err != nil || len(snapshots) != 2 || record.GetString("baseline") != baselineID || record.GetBool("config_available") || record.GetString("notes") != "Keep this note" || record.GetString("room") != "Bedroom" || !strings.Contains(response.Header().Get("Location"), "Baseline+unchanged") {
		t.Fatalf("failed approval changed saved data or appeared successful: record=%v history=%v err=%v", record, snapshots, err)
	}
}
