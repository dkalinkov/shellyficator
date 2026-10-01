package shelly

import (
	"net/netip"
	"strings"
	"testing"
)

func TestCachedNeighborsOnlyOffersKnownHostsOnEligibleLANs(t *testing.T) {
	prefixes := map[string][]netip.Prefix{"eth3": {netip.MustParsePrefix("192.168.1.10/24")}}
	linux := `IP address       HW type     Flags       HW address            Mask     Device
192.168.1.31    0x1         0x2         02:00:00:00:00:0C     *        eth3
192.168.1.31    0x1         0x2         02:00:00:00:00:0C     *        eth3
192.168.1.32    0x1         0x0         00:00:00:00:00:00     *        eth3
192.168.1.33    0x1         0x2         02:00:00:00:00:0C     *        docker0
192.168.1.10    0x1         0x2         02:00:00:00:00:0C     *        eth3
192.168.1.255   0x1         0x2         ff:ff:ff:ff:ff:ff     *        eth3
192.168.2.31    0x1         0x2         02:00:00:00:00:0C     *        eth3
8.8.8.8        0x1         0x2         02:00:00:00:00:0C     *        eth3
`
	windows := `Interface: 192.168.1.10 --- 0x6
  Internet Address      Physical Address      Type
  192.168.1.31           02-00-00-00-00-0C     dynamic
  192.168.1.31           02-00-00-00-00-0C     dynamic
  192.168.1.32           00-00-00-00-00-00     invalid
  192.168.1.10           02-00-00-00-00-0C     dynamic
  192.168.1.255          ff-ff-ff-ff-ff-ff     static
  192.168.2.31           02-00-00-00-00-0C     dynamic
  224.0.0.251           01-00-5e-00-00-fb     static
`
	for _, test := range []struct {
		name, cache string
		windows     bool
	}{{"linux", linux, false}, {"windows", windows, true}} {
		t.Run(test.name, func(t *testing.T) {
			addresses := cachedNeighbors(strings.NewReader(test.cache), prefixes, test.windows)
			if len(addresses) != 1 || addresses[0] != "http://192.168.1.31" {
				t.Fatalf("unexpected cached candidates: %v", addresses)
			}
		})
	}
}
