package shelly

import (
	"encoding/json"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

// Run one target with: go test ./internal/shelly -run=^$ -fuzz=^FuzzTargets$

func FuzzTargets(f *testing.F) {
	for _, seed := range []string{
		"192.168.1.0/24", "192.168.1.20/30", "10.0.0.7/32", "172.16.4.0/23",
		"8.8.8.0/24", "fd00::/120", "192.168.1.20", "shelly-plus.local",
		"http://192.168.1.20/", "https://user:pass@192.168.1.20", "http://h/x?y#z",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		addresses, err := Targets(value)
		if err != nil {
			return
		}
		if len(addresses) == 0 || len(addresses) > 256 {
			t.Fatalf("Targets(%q) returned %d addresses", value, len(addresses))
		}
		if !strings.Contains(value, "/") || strings.Contains(value, "://") {
			// A single address must stay a normalized credential-free origin.
			again, err := NormalizeAddress(addresses[0])
			if err != nil || again != addresses[0] {
				t.Fatalf("NormalizeAddress(%q) = %q, %v; want it unchanged", addresses[0], again, err)
			}
			if u, _ := url.Parse(addresses[0]); u.User != nil {
				t.Fatalf("Targets(%q) kept credentials: %q", value, addresses[0])
			}
			return
		}
		prefix := netip.MustParsePrefix(strings.TrimSpace(value)).Masked()
		for _, address := range addresses {
			ip, err := netip.ParseAddr(strings.TrimPrefix(address, "http://"))
			if err != nil || !strings.HasPrefix(address, "http://") || !ip.Is4() || !ip.IsPrivate() || !prefix.Contains(ip) {
				t.Fatalf("Targets(%q) returned %q outside the private range", value, address)
			}
			if prefix.Bits() <= 30 && ip == prefix.Addr() {
				t.Fatalf("Targets(%q) returned the network address %q", value, address)
			}
		}
	})
}

func FuzzRedact(f *testing.F) {
	for _, seed := range []string{
		`{"wifi":{"sta":{"ssid":"home","pass":"secret"}}}`,
		`{"mqtt":{"server":"mqtt://user:secret@broker.local:1883","Token":"t"}}`,
		`{"cloud":{"servers":[{"url":"https://a:b@example.com/x"},"ws://c@d"]}}`,
		`{"auth":{"ha1":"abc","API_KEY":"k","private_key":null,"client_key":[1,2]}}`,
		`{"name":"//user@host","note":"no secrets here"}`,
	} {
		f.Add([]byte(seed))
	}
	sensitive := map[string]bool{"pass": true, "password": true, "token": true, "ha1": true, "auth_key": true, "api_key": true, "private_key": true, "client_key": true}
	f.Fuzz(func(t *testing.T, data []byte) {
		var config map[string]any
		if json.Unmarshal(data, &config) != nil {
			return
		}
		Redact(config)
		var check func(path string, value any)
		check = func(path string, value any) {
			switch value := value.(type) {
			case map[string]any:
				for key, entry := range value {
					if sensitive[strings.ToLower(key)] && entry != "[redacted]" {
						t.Fatalf("%s/%s kept %#v; input %s", path, key, entry, data)
					}
					check(path+"/"+key, entry)
				}
			case []any:
				for _, entry := range value {
					check(path+"[]", entry)
				}
			case string:
				if u, err := url.Parse(value); err == nil && u.User != nil {
					t.Fatalf("%s kept URL credentials in %q; input %s", path, value, data)
				}
			}
		}
		check("", config)
	})
}

func FuzzCachedNeighbors(f *testing.F) {
	f.Add(`IP address       HW type     Flags       HW address            Mask     Device
192.168.1.20    0x1         0x2         02:00:00:00:00:01     *        eth0
192.168.1.10    0x1         0x2         02:00:00:00:00:02     *        eth0
10.0.3.4        0x1         0x2         02:00:00:00:00:03     *        wlan0
192.168.1.33    0x1         0x2         02:00:00:00:00:04     *        docker0
`, false)
	f.Add(`Interface: 192.168.1.10 --- 0x4
  Internet Address      Physical Address      Type
  192.168.1.20          02-00-00-00-00-01     dynamic
  192.168.1.255         ff-ff-ff-ff-ff-ff     static
  10.0.0.0              02-00-00-00-00-05     dynamic
`, true)
	prefixes := map[string][]netip.Prefix{
		"eth0":  {netip.MustParsePrefix("192.168.1.10/24")},
		"wlan0": {netip.MustParsePrefix("10.0.0.5/16")},
	}
	f.Fuzz(func(t *testing.T, table string, windows bool) {
		addresses := cachedNeighbors(strings.NewReader(table), prefixes, windows)
		if len(addresses) > 128 {
			t.Fatalf("returned %d addresses, more than the limit of 128", len(addresses))
		}
		seen := map[string]bool{}
		for _, address := range addresses {
			if seen[address] {
				t.Fatalf("returned %q twice", address)
			}
			seen[address] = true
			ip, err := netip.ParseAddr(strings.TrimPrefix(address, "http://"))
			if err != nil || !strings.HasPrefix(address, "http://") || !ip.Is4() || !ip.IsPrivate() {
				t.Fatalf("returned %q, which is not a private IPv4 address", address)
			}
			local := false
			for _, ranges := range prefixes {
				for _, prefix := range ranges {
					local = local || (prefix.Contains(ip) && ip != prefix.Addr() && ip != prefix.Masked().Addr())
				}
			}
			if !local {
				t.Fatalf("returned %q, which is not a neighbor on an eligible interface", address)
			}
		}
	})
}
