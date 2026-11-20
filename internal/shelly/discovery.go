package shelly

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"

	"shellyficator/internal/domain"
)

type NetworkDiscovery struct{}

func (NetworkDiscovery) LocalNetworks() ([]domain.Network, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var networks []domain.Network
	seen := map[string]bool{}
	for _, iface := range interfaces {
		if !discoveryInterface(iface) {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			rangeName := discoveryPrefix(address.String())
			if rangeName == "" {
				continue
			}
			if !seen[rangeName] {
				networks = append(networks, domain.Network{Interface: iface.Name, Prefix: rangeName})
				seen[rangeName] = true
			}
		}
	}
	sort.Slice(networks, func(i, j int) bool { return networks[i].Prefix < networks[j].Prefix })
	if len(networks) > 4 {
		networks = networks[:4]
	}
	return networks, nil
}

func discoveryPrefix(value string) string {
	prefix, err := netip.ParsePrefix(value)
	if err != nil || !prefix.Addr().Is4() || !prefix.Addr().IsPrivate() || prefix.Bits() < 24 || prefix.Bits() > 30 {
		return ""
	}
	return prefix.Masked().String()
}

func discoveryInterface(iface net.Interface) bool {
	if iface.Flags&net.FlagUp == 0 || iface.Flags&(net.FlagLoopback|net.FlagPointToPoint) != 0 || iface.Flags&net.FlagMulticast == 0 {
		return false
	}
	for _, prefix := range []string{"docker", "br-", "veth", "virbr", "cni", "flannel", "tailscale", "tun", "wg"} {
		if strings.HasPrefix(strings.ToLower(iface.Name), prefix) {
			return false
		}
	}
	return true
}

// Announcements returns advertised candidates; /shelly still verifies each one.
func (NetworkDiscovery) Announcements(ctx context.Context) ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	entries := make(chan *mdns.ServiceEntry, 256)
	var queries sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	for _, iface := range interfaces {
		if !discoveryInterface(iface) {
			continue
		}
		for _, service := range []string{"_shelly._tcp", "_http._tcp"} {
			queries.Go(func() {
				err := mdns.QueryContext(ctx, &mdns.QueryParam{Service: service, Domain: "local", Timeout: 3 * time.Second, Interface: &iface, Entries: entries, WantUnicastResponse: true, DisableIPv6: true, Logger: log.New(io.Discard, "", 0)})
				if err != nil {
					mu.Lock()
					failures = append(failures, err)
					mu.Unlock()
				}
			})
		}
	}
	go func() { queries.Wait(); close(entries) }()
	seen := map[string]bool{}
	var addresses []string
	for entry := range entries {
		address := announcementAddress(entry)
		if address != "" && !seen[address] && len(addresses) < 256 {
			seen[address] = true
			addresses = append(addresses, address)
		}
	}
	sort.Strings(addresses)
	return addresses, errors.Join(failures...)
}

func announcementAddress(entry *mdns.ServiceEntry) string {
	if entry == nil || entry.Port < 1 || entry.Port > 65535 || entry.AddrV4.To4() == nil || !entry.AddrV4.IsPrivate() {
		return ""
	}
	name := strings.ToLower(entry.Name)
	if !strings.HasSuffix(name, "._shelly._tcp.local.") {
		if !strings.HasSuffix(name, "._http._tcp.local.") {
			return ""
		}
		supported := false
		for _, field := range entry.InfoFields {
			if value, ok := strings.CutPrefix(field, "gen="); ok {
				generation, err := strconv.Atoi(value)
				supported = err == nil && generation >= 2
			}
		}
		if !supported {
			return ""
		}
	}
	address := "http://" + entry.AddrV4.String()
	if entry.Port != 80 {
		address += fmt.Sprintf(":%d", entry.Port)
	}
	return address
}
