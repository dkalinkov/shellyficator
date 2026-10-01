package shelly

import (
	"net"
	"testing"

	"github.com/hashicorp/mdns"
)

func TestSuggestedDiscoveryNetworkBounds(t *testing.T) {
	for _, value := range []string{"192.168.1.10/24", "10.0.0.9/28"} {
		if discoveryPrefix(value) == "" {
			t.Errorf("did not detect eligible LAN %s", value)
		}
	}
	for _, value := range []string{"172.17.0.2/16", "192.168.1.10/32", "8.8.8.8/24", "169.254.10.5/24", "fd00::1/64", "invalid"} {
		if discoveryPrefix(value) != "" {
			t.Errorf("automatically selected unsuitable network %s", value)
		}
	}
	up := net.FlagUp | net.FlagMulticast
	if !discoveryInterface(net.Interface{Name: "eth3", Flags: up}) {
		t.Fatal("ignored active LAN interface")
	}
	for _, iface := range []net.Interface{{Name: "eth0"}, {Name: "lo", Flags: up | net.FlagLoopback}, {Name: "eth1", Flags: up | net.FlagPointToPoint}, {Name: "docker0", Flags: up}, {Name: "br-abcdef", Flags: up}, {Name: "veth123", Flags: up}, {Name: "tailscale0", Flags: up}} {
		if discoveryInterface(iface) {
			t.Errorf("selected unsuitable interface %s", iface.Name)
		}
	}
}

func TestAnnouncementsRequireShellyServiceOrGeneration(t *testing.T) {
	tests := []struct {
		name, ip string
		fields   []string
		port     int
		want     string
	}{
		{"light._shelly._tcp.local.", "192.168.1.31", nil, 80, "http://192.168.1.31"},
		{"light._http._tcp.local.", "192.168.1.32", []string{"gen=4"}, 8080, "http://192.168.1.32:8080"},
		{"printer._http._tcp.local.", "192.168.1.33", nil, 80, ""},
		{"old._http._tcp.local.", "192.168.1.34", []string{"gen=1"}, 80, ""},
		{"light._shelly._tcp.local.", "8.8.8.8", nil, 80, ""},
		{"light._shelly._tcp.local.", "127.0.0.1", nil, 80, ""},
		{"light._shelly._tcp.local.", "192.168.1.31", nil, 0, ""},
		{"light._ssh._tcp.local.", "192.168.1.31", []string{"gen=4"}, 80, ""},
	}
	for _, test := range tests {
		entry := &mdns.ServiceEntry{Name: test.name, AddrV4: net.ParseIP(test.ip), InfoFields: test.fields, Port: test.port}
		if got := announcementAddress(entry); got != test.want {
			t.Errorf("announcement %s: got %q, want %q", test.name, got, test.want)
		}
	}
}

func TestAutomaticDiscoveryOnlyUsesAnnouncedAddresses(t *testing.T) {
	addresses := DiscoveryAddresses([]string{"http://192.168.1.2", "http://192.168.1.2:8080", "http://192.168.1.2"})
	if len(addresses) != 2 || addresses[0] != "http://192.168.1.2" || addresses[1] != "http://192.168.1.2:8080" {
		t.Fatalf("unexpected automatic discovery addresses: %v", addresses)
	}
	if addresses := DiscoveryAddresses(nil); len(addresses) != 0 {
		t.Fatalf("empty announcements expanded into addresses: %v", addresses)
	}
}
