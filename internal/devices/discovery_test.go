package devices

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEmptyAutomaticDiscoveryStopsWithoutProbingAddresses(t *testing.T) {
	for _, queryError := range []error{nil, errors.New("multicast unavailable")} {
		s := New(nil, nil)
		t.Cleanup(s.Close)
		s.discovery = Discovery{Running: true}
		s.announcements = func(context.Context) ([]string, error) { return nil, queryError }
		s.neighbors = func(context.Context) ([]string, error) { return nil, nil }
		// No client/store: empty discovery must finish without making device requests.
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
	var requests atomic.Int32
	s, device := testService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	s.discovery = Discovery{Running: true}
	s.announcements = func(context.Context) ([]string, error) { return []string{device.URL, device.URL}, nil }
	s.neighbors = func(context.Context) ([]string, error) { return []string{device.URL}, nil }
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
	s, device := testService(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"cached-device","mac":"AABBCCDDEEFF","gen":4}`))
		default:
			_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"momentary"}}`))
		}
	}))
	s.discovery = Discovery{Running: true}
	s.announcements = func(context.Context) ([]string, error) { return nil, errors.New("multicast unavailable") }
	s.neighbors = func(context.Context) ([]string, error) { return []string{device.URL}, nil }
	s.runDiscovery(true, nil)
	if s.discovery.Found != 1 || s.discovery.Total != 1 || s.discovery.Running {
		t.Fatalf("cached device not discovered: %+v", s.discovery)
	}
	records, err := s.store.Devices()
	if err != nil || len(records) != 1 || records[0].GetString("device_id") != "cached-device" {
		t.Fatalf("cached identity not saved: count=%d err=%v", len(records), err)
	}
}
