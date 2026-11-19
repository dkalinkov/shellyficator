package devices

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"shellyficator/internal/domain"
	"shellyficator/internal/shelly"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEmptyAutomaticDiscoveryStopsWithoutProbingAddresses(t *testing.T) {
	for _, queryError := range []error{nil, errors.New("multicast unavailable")} {
		s := New(nil, nil, Options{Discovery: shelly.NetworkDiscovery{}})
		t.Cleanup(s.Close)
		s.discovery = domain.Discovery{Running: true}
		s.announcements = func(context.Context) ([]string, error) { return nil, queryError }
		s.neighbors = func(context.Context) ([]string, error) { return nil, nil }
		// No client/store: empty discovery must finish without making device requests.
		s.runDiscovery(true, nil)
		if s.discovery.Running || s.discovery.Total != 0 || s.discovery.Completed != 0 || s.discovery.Found != 0 {
			t.Fatalf("empty announcements started address probes: %+v", s.discovery)
		}
		if !strings.Contains(s.discovery.Message, "multicast") || !strings.Contains(s.discovery.Message, "Advanced discovery") || strings.Contains(s.discovery.Message, "Discovery complete:") {
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
	s.discovery = domain.Discovery{Running: true}
	s.announcements = func(context.Context) ([]string, error) { return []string{device.URL, device.URL}, nil }
	s.neighbors = func(context.Context) ([]string, error) { return []string{device.URL}, nil }
	s.runDiscovery(true, nil)
	if s.discovery.Running || s.discovery.Total != 1 || s.discovery.Completed != 1 || s.discovery.Found != 1 || requests.Load() != 3 {
		t.Fatalf("discovery checked more than the announced device: job=%+v requests=%d", s.discovery, requests.Load())
	}
	records, err := s.store.Devices()
	if err != nil || len(records) != 1 || !records[0].ConfigAvailable {
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
	s.discovery = domain.Discovery{Running: true}
	s.announcements = func(context.Context) ([]string, error) { return nil, errors.New("multicast unavailable") }
	s.neighbors = func(context.Context) ([]string, error) { return []string{device.URL}, nil }
	s.runDiscovery(true, nil)
	if s.discovery.Found != 1 || s.discovery.Total != 1 || s.discovery.Running {
		t.Fatalf("cached device not discovered: %+v", s.discovery)
	}
	records, err := s.store.Devices()
	if err != nil || len(records) != 1 || records[0].DeviceID != "cached-device" {
		t.Fatalf("cached identity not saved: count=%d err=%v", len(records), err)
	}
}

func closedAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Fatal(err)
	}
	address := "http://" + listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func fakeDeviceHandler(id string, requests *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"` + id + `","mac":"AABBCCDDEEFF","gen":4}`))
		default:
			_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"momentary"}}`))
		}
	}
}

type networkSource struct {
	shelly.NetworkDiscovery
	targets map[string][]string
}

func (n networkSource) Targets(target string) ([]string, error) {
	if addresses, ok := n.targets[target]; ok {
		return addresses, nil
	}
	return nil, errors.New("unknown test network")
}

func TestSubnetScanVerifiesOnlyAddressesThatAcceptConnections(t *testing.T) {
	var requests atomic.Int32
	s, device := testService(t, fakeDeviceHandler("scanned-device", &requests))
	addresses := []string{closedAddress(t), device.URL, closedAddress(t)}
	s.discovery = domain.Discovery{Running: true, Target: "loopback test", Total: len(addresses)}
	s.runDiscovery(false, addresses)
	if s.discovery.Found != 1 || s.discovery.Total != 1 || requests.Load() != 3 {
		t.Fatalf("scan did not verify exactly the open address: job=%+v requests=%d", s.discovery, requests.Load())
	}
}

func TestSubnetScanRemembersNetworksThatFoundDevices(t *testing.T) {
	var requests atomic.Int32
	s, device := testService(t, fakeDeviceHandler("remembered-device", &requests))
	empty := closedAddress(t)

	s.discovery = domain.Discovery{Running: true, Target: "192.168.51.0/24"}
	s.runDiscovery(false, []string{empty, closedAddress(t)})
	if networks, _ := s.store.DiscoveryNetworks(); len(networks) != 0 {
		t.Fatalf("a scan without devices was remembered: %v", networks)
	}

	s.discovery = domain.Discovery{Running: true, Target: "192.168.52.5"}
	s.runDiscovery(false, []string{device.URL})
	if networks, _ := s.store.DiscoveryNetworks(); len(networks) != 0 {
		t.Fatalf("a single-address scan was remembered: %v", networks)
	}

	s.discovery = domain.Discovery{Running: true, Target: "192.168.50.0/24"}
	s.runDiscovery(false, []string{empty, device.URL})
	networks, err := s.store.DiscoveryNetworks()
	if err != nil || len(networks) != 1 || networks[0] != "192.168.50.0/24" {
		t.Fatalf("scan with devices was not remembered: %v %v", networks, err)
	}
	if !strings.Contains(s.discovery.Message, "192.168.50.0/24") {
		t.Fatalf("message does not mention the remembered network: %q", s.discovery.Message)
	}

	for i := 1; i < domain.MaxDiscoveryNetworks; i++ {
		if err := s.store.RememberNetwork(fmt.Sprintf("192.168.%d.0/24", 200+i)); err != nil {
			t.Fatal(err)
		}
	}
	s.discovery = domain.Discovery{Running: true, Target: "192.168.99.0/24"}
	s.runDiscovery(false, []string{empty, device.URL})
	if !strings.Contains(s.discovery.Message, "Not remembered") {
		t.Fatalf("network limit was not explained: %q", s.discovery.Message)
	}
}

func TestAutomaticDiscoveryScansRememberedNetworks(t *testing.T) {
	var requests atomic.Int32
	s, device := testService(t, fakeDeviceHandler("vlan-device", &requests))
	s.source = networkSource{targets: map[string][]string{"192.168.60.0/24": {closedAddress(t), device.URL, closedAddress(t)}}}
	if err := s.store.RememberNetwork("192.168.60.0/24"); err != nil {
		t.Fatal(err)
	}
	s.discovery = domain.Discovery{Running: true}
	s.announcements = func(context.Context) ([]string, error) { return nil, nil }
	s.neighbors = func(context.Context) ([]string, error) { return nil, nil }
	s.runDiscovery(true, nil)
	if s.discovery.Found != 1 || s.discovery.Total != 1 || requests.Load() != 3 {
		t.Fatalf("remembered network was not scanned: job=%+v requests=%d", s.discovery, requests.Load())
	}

	requests.Store(0)
	s.discovery = domain.Discovery{Running: true}
	s.announcements = func(context.Context) ([]string, error) { return []string{device.URL}, nil }
	s.runDiscovery(true, nil)
	if s.discovery.Total != 1 || requests.Load() != 3 {
		t.Fatalf("announced device was checked twice: job=%+v requests=%d", s.discovery, requests.Load())
	}
}
