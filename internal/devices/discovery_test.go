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
	"time"
)

func TestRediscoveryWinsOverInFlightReadsWithoutAddingHistory(t *testing.T) {
	for _, operation := range []string{"poll success", "poll identity mismatch", "approve baseline"} {
		t.Run(operation, func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			const newAddress = "http://192.0.2.2"
			var device domain.Device
			s, repository, saved := workflowFixture(t, workflowClient{
				read: func(ctx context.Context, _ string, _ string, _ bool) (domain.Reading, error) {
					close(started)
					select {
					case <-release:
					case <-ctx.Done():
						return domain.Reading{}, ctx.Err()
					}
					if operation == "poll identity mismatch" {
						return domain.Reading{}, domain.ErrIdentity
					}
					return deviceReading(device), nil
				},
				discover: func(_ context.Context, address string) (domain.Reading, error) {
					reading := deviceReading(device)
					reading.Address = address
					return reading, nil
				},
			})
			device = saved
			s.source = networkSource{targets: map[string][]string{newAddress: {newAddress}}}
			var poll <-chan struct{}
			approval := make(chan error, 1)
			if operation == "approve baseline" {
				go func() { _, err := s.ApproveBaseline(context.Background(), device.ID); approval <- err }()
			} else {
				poll = s.Refresh(false)
			}
			waitForJob(t, started)
			if err := s.Discover(newAddress, false); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(time.Second)
			for s.Discovery().Running && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if found := s.Discovery(); found.Running || found.Found != 1 {
				t.Fatalf("rediscovery did not complete: %+v", found)
			}
			close(release)
			if poll != nil {
				waitForJob(t, poll)
			} else if err := <-approval; !errors.Is(err, domain.ErrDeviceMoved) {
				t.Fatalf("approval accepted an old address: %v", err)
			}
			updated, _ := repository.Device(device.ID)
			snapshots, _ := repository.Snapshots(device.ID)
			if updated.Address != newAddress || updated.IdentityMismatch || updated.BaselineID != device.BaselineID || len(snapshots) != 0 {
				t.Fatalf("rediscovery lost current state or changed history: %+v", updated)
			}
		})
	}
}

func TestEmptyAutomaticDiscoveryStopsWithoutProbingAddresses(t *testing.T) {
	for _, queryError := range []error{nil, errors.New("multicast unavailable")} {
		s := New(nil, nil, Options{Discovery: networkSource{announcementErr: queryError}})
		t.Cleanup(s.Close)
		s.discovery = domain.Discovery{Running: true}
		// No client/store: empty discovery must finish without making device requests.
		s.runDiscovery(true, nil)
		if s.discovery.Running || s.discovery.Total != 0 || s.discovery.Completed != 0 || s.discovery.Found != 0 {
			t.Fatalf("empty announcements started address probes: %+v", s.discovery)
		}
		if !strings.Contains(s.discovery.Message, "multicast") || !strings.Contains(s.discovery.Message, "Advanced discovery") {
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
	s.source = networkSource{announced: []string{device.URL, device.URL}, neighbors: []string{device.URL}}
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
	s.source = networkSource{announcementErr: errors.New("multicast unavailable"), neighbors: []string{device.URL}}
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
	targets              map[string][]string
	announced, neighbors []string
	announcementErr      error
	query                func(context.Context) ([]string, error)
}

func (n networkSource) Announcements(ctx context.Context) ([]string, error) {
	if n.query != nil {
		return n.query(ctx)
	}
	return n.announced, n.announcementErr
}

func (n networkSource) Neighbors(ctx context.Context) ([]string, error) {
	if n.query != nil {
		return n.query(ctx)
	}
	return n.neighbors, nil
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
	source := networkSource{targets: map[string][]string{"192.168.60.0/24": {closedAddress(t), device.URL, closedAddress(t)}}}
	s.source = source
	if err := s.store.RememberNetwork("192.168.60.0/24"); err != nil {
		t.Fatal(err)
	}
	s.discovery = domain.Discovery{Running: true}
	s.runDiscovery(true, nil)
	if s.discovery.Found != 1 || s.discovery.Total != 1 || requests.Load() != 3 {
		t.Fatalf("remembered network was not scanned: job=%+v requests=%d", s.discovery, requests.Load())
	}

	requests.Store(0)
	s.discovery = domain.Discovery{Running: true}
	source.announced = []string{device.URL}
	s.source = source
	s.runDiscovery(true, nil)
	if s.discovery.Total != 1 || requests.Load() != 3 {
		t.Fatalf("announced device was checked twice: job=%+v requests=%d", s.discovery, requests.Load())
	}
}
