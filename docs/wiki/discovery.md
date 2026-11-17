# Device discovery

Shellyficator targets Gen2+ devices. It validates candidates through `/shelly`
and reads their supported RPC interfaces. A device's reported ID/MAC establishes
identity; an IP address alone does not. Rediscovery preserves rooms, tags,
notes, configuration history, and approved baselines.

## Normal discovery

**Discover** combines IPv4 mDNS advertisements with cached LAN neighbors. The
neighbor adapter reads Linux ARP entries and Windows neighbor output, including
Windows cache access under WSL when available. Candidates are restricted to
eligible local networks and validated before entering inventory. Normal discovery
does not sweep every address in a subnet.

## Advanced discovery

Supply a device IP/hostname, or explicitly scan a private IPv4 subnet no larger
than `/24`. For example, `192.168.50.0/24` is a sample LAN range; substitute your
actual device network. Suggested networks come from directly connected eligible
interfaces. A scan makes requests to individual addresses and may take minutes.

The server must reach the devices. Your browser also needs device-network access
when following **Open device UI**, which opens the device's own interface.

## When devices are missing

- Try an explicit host first. Empty mDNS results can mean multicast is blocked,
  particularly across VLANs, WSL, or container networks.
- Check routing and firewall access from the server, not only from your laptop.
- Use the shared password setting for protected devices; authentication failures
  differ from connectivity failures.
- Try a bounded subnet scan when multicast and neighbor caches have no candidates.
- For native Linux containers, host networking usually fits LAN discovery. Bridge
  networking needs a reachable LAN and explicit discovery targets.

Avoid increasing scan concurrency to compensate for missing responses: aggressive
sweeps produced false negatives during local testing. Verify discovery from the
actual deployment host.

A failed request does not remove a known device or replace its identity. See
[status and stale readings](usage.md#refresh-and-stale-readings) for offline rules.
