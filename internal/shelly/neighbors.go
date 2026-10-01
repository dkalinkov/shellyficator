package shelly

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Neighbors offers cached LAN addresses for identity checks, without a subnet sweep.
// On WSL the Windows cache is also useful: it can contain devices that the guest
// has not resolved yet. This only reads the cache; it does not change networking.
func Neighbors(ctx context.Context) ([]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	prefixes := map[string][]netip.Prefix{}
	for _, iface := range interfaces {
		if !discoveryInterface(iface) {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			prefix, err := netip.ParsePrefix(address.String())
			if err == nil && prefix.Addr().Is4() && prefix.Addr().IsPrivate() {
				prefixes[iface.Name] = append(prefixes[iface.Name], prefix)
			}
		}
	}
	var addresses []string
	var failures []error
	if runtime.GOOS == "linux" {
		file, err := os.Open("/proc/net/arp")
		if err == nil {
			addresses = cachedNeighbors(file, prefixes, false)
			_ = file.Close()
		} else {
			failures = append(failures, err)
		}
	}
	command := ""
	if runtime.GOOS == "windows" {
		command = "arp.exe"
	} else if kernel, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil && strings.Contains(strings.ToLower(string(kernel)), "microsoft") {
		if path, err := exec.LookPath("arp.exe"); err == nil {
			command = path
		}
	}
	if command != "" {
		output, err := exec.CommandContext(ctx, command, "-a").Output()
		if err == nil {
			addresses = append(addresses, cachedNeighbors(strings.NewReader(string(output)), prefixes, true)...)
		} else {
			failures = append(failures, err)
		}
	}
	addresses = DiscoveryAddresses(addresses)
	probeNeighbors(ctx, addresses)
	return addresses, errors.Join(failures...)
}

// Unicast mDNS reaches hosts when multicast browsing is unavailable and starts
// neighbor resolution before the HTTP identity request. Replies are hints only;
// the caller still verifies every candidate through /shelly.
func probeNeighbors(ctx context.Context, addresses []string) {
	limit := make(chan struct{}, 8)
	var group sync.WaitGroup
	for _, address := range addresses {
		select {
		case limit <- struct{}{}:
		case <-ctx.Done():
			group.Wait()
			return
		}
		group.Add(1)
		go func(address string) {
			defer func() { <-limit; group.Done() }()
			ip := strings.TrimPrefix(address, "http://")
			question := new(dns.Msg)
			question.SetQuestion("_shelly._tcp.local.", dns.TypePTR)
			question.Id = 0
			question.RecursionDesired = false
			question.Question[0].Qclass |= 1 << 15
			client := dns.Client{Net: "udp", Timeout: 500 * time.Millisecond}
			_, _, _ = client.ExchangeContext(ctx, question, net.JoinHostPort(ip, "5353"))
		}(address)
	}
	group.Wait()
}

func cachedNeighbors(reader io.Reader, prefixes map[string][]netip.Prefix, windows bool) []string {
	var addresses []string
	seen := map[string]bool{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		ip, err := netip.ParseAddr(fields[0])
		if err != nil || !ip.Is4() || !ip.IsPrivate() {
			continue
		}
		mac := fields[1]
		eligible := prefixes
		if !windows {
			if len(fields) != 6 {
				continue
			}
			flags, err := strconv.ParseUint(fields[2], 0, 32)
			if err != nil || flags&2 == 0 {
				continue
			}
			mac = fields[3]
			eligible = map[string][]netip.Prefix{fields[5]: prefixes[fields[5]]}
		}
		hardware, err := net.ParseMAC(strings.ReplaceAll(mac, "-", ":"))
		if err != nil || len(hardware) != 6 || hardware[0]&1 != 0 || hardware.String() == "00:00:00:00:00:00" {
			continue
		}
		local := false
		for _, ranges := range eligible {
			for _, prefix := range ranges {
				if prefix.Contains(ip) && prefix.Addr() != ip && prefix.Masked().Addr() != ip {
					local = true
				}
			}
		}
		if local && !seen[ip.String()] && len(addresses) < 128 {
			seen[ip.String()] = true
			addresses = append(addresses, "http://"+ip.String())
		}
	}
	return addresses
}
