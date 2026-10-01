# LAN discovery research

Discovery must be verified from the deployment host. Multicast can be unavailable
under virtualization, across VLANs, or in container networks. An empty mDNS result
is not evidence that no devices exist.

## Findings retained from local testing

- Combining cached neighbors, mDNS, and paced HTTP requests found devices missed
  by multicast alone. Aggressive concurrent sweeps produced false negatives.
- Cached neighbors are candidates, not identities. Validate each host with
  `/shelly` and preserve its reported ID/MAC across address changes.
- Failed reads must preserve inventory, local metadata, captures, and baselines.
- Keep explicit subnet scans bounded and separate from automatic discovery.
- Verify container networking on the actual deployment host; Docker Desktop and
  native Linux host networking can behave differently.

The original household inventory and screenshot were removed for publication.
See the [discovery guide](../docs/wiki/discovery.md) for current behavior and
troubleshooting. Gen1 support is not implemented.
