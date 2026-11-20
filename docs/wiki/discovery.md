# Device discovery

**Discover** checks IPv4 mDNS announcements, cached LAN neighbors, and up to four
remembered subnets. On WSL it can also read the Windows ARP cache. It verifies each
candidate through `/shelly`. Rediscovery updates the address and readings while
preserving rooms, tags, notes, captures, and baselines.

## Advanced discovery

Enter a host or private IPv4 subnet no larger than `/24`, such as
`192.168.50.0/24`. Subnet scans skip hosts that refuse a TCP connection.
Shellyficator remembers subnets that contain devices. Use **Forget** to remove one.

## When devices are missing

- Try the device's address directly from the server. Multicast can be blocked
  across VLANs, WSL, or container networks.
- Set the shared password for protected devices. For containers, see
  [networking options](deployment.md#linux-container).
- **Open device UI** also requires the browser to reach the device.

See [refresh and stale readings](usage.md#refresh-and-stale-readings) for saved devices.
