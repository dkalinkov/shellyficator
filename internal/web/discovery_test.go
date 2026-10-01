package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pocketbase/pocketbase"
	"shelly-manager/internal/inventory"
	"shelly-manager/internal/shelly"
	_ "shelly-manager/migrations"
)

func TestEmptyAutomaticDiscoveryStopsWithoutProbingAddresses(t *testing.T) {
	for _, queryError := range []error{nil, errors.New("multicast unavailable")} {
		s := &Server{
			discovery:     Discovery{Running: true},
			announcements: func(context.Context) ([]string, error) { return nil, queryError },
			// No client/store: empty discovery must finish without making device requests.
		}
		s.runDiscovery(true, nil)
		if s.discovery.Running || s.discovery.Total != 0 || s.discovery.Completed != 0 || s.discovery.Found != 0 {
			t.Fatalf("empty announcements started address probes: %+v", s.discovery)
		}
		if !strings.Contains(s.discovery.Message, "multicast") || strings.Contains(s.discovery.Message, "Discovery complete:") {
			t.Fatalf("missing network visibility explanation: %s", s.discovery.Message)
		}
	}
}

func TestAutomaticDiscoveryOnlyVerifiesAnnouncedDevices(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	device := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{"switch:0":{"output":false}}`))
		case "/rpc/Shelly.GetConfig":
			_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"momentary"}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer device.Close()
	s := &Server{
		store: inventory.New(app), client: shelly.NewClient(""),
		discovery: Discovery{Running: true},
		announcements: func(context.Context) ([]string, error) {
			return []string{device.URL, device.URL}, nil
		},
		neighbors: func(context.Context) ([]string, error) {
			return []string{device.URL}, nil
		},
	}
	s.runDiscovery(true, nil)
	if s.discovery.Running || s.discovery.Total != 1 || s.discovery.Completed != 1 || s.discovery.Found != 1 || requests.Load() != 3 {
		t.Fatalf("discovery checked more than the announced device: job=%+v requests=%d", s.discovery, requests.Load())
	}
	records, err := s.store.Devices()
	if err != nil || len(records) != 1 || !records[0].GetBool("config_available") {
		t.Fatalf("identified device/configuration not saved: count=%d err=%v", len(records), err)
	}
}

func TestDiscoveryVerifiesCachedNeighborWhenMulticastIsUnavailable(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	device := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"cached-device","mac":"AABBCCDDEEFF","gen":4}`))
		default:
			_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"momentary"}}`))
		}
	}))
	defer device.Close()
	s := &Server{
		store: inventory.New(app), client: shelly.NewClient(""),
		discovery: Discovery{Running: true},
		announcements: func(context.Context) ([]string, error) {
			return nil, errors.New("multicast unavailable")
		},
		neighbors: func(context.Context) ([]string, error) { return []string{device.URL}, nil },
	}
	s.runDiscovery(true, nil)
	if s.discovery.Found != 1 || s.discovery.Total != 1 || s.discovery.Running {
		t.Fatalf("cached device not discovered: %+v", s.discovery)
	}
	records, err := s.store.Devices()
	if err != nil || len(records) != 1 || records[0].GetString("device_id") != "cached-device" {
		t.Fatalf("cached identity not saved: count=%d err=%v", len(records), err)
	}
}
