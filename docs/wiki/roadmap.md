# Roadmap

Shellyficator focuses on local Gen2+ device inventory and configuration changes
that can affect physical controls. See the [README](../../README.md#what-it-does)
for delivered features and the [usage guide](usage.md) for monitoring coverage.
The items below are planned work, not available features or release commitments.

## Next priorities

- Validate discovery, status reads, configuration captures, and native Linux
  container networking on deployment hosts, including physical-control checks.
- Add switch/input configuration editing with capability checks, a capture before
  each write, and readback verification. Test momentary and toggle controls on
  real hardware before claiming physical-control compatibility.
- Add manual and scheduled issue scans with persistent reports, per-device
  outcomes, and configurable frequency, time, and timezone. Cover configuration
  drift, reachability, disabled authentication, and reported faults; distinguish
  authentication failures from connectivity failures.
- Add encrypted stored credentials and selected-device password changes. Use a
  deployment-supplied encryption key and exclude credentials from snapshots,
  logs, and links. Validate a single-device rollout through its own UI,
  Shellyficator, and Home Assistant before expanding; track partial failures.
- Add persisted refresh intervals, access-point/timer status, and settings filters.

## Longer-term ideas

Selected configuration restoration, profiles, network/MQTT configuration,
script/schedule/webhook editing, Wi-Fi provisioning, repeatable notifications,
individual device passwords, and Home Assistant app packaging.

Gen1 support is outside the current scope. Firmware installation is already
available; see [its guide](firmware.md) for verification and recovery behavior.
Discuss larger changes in an issue before implementation, as described in
[Contributing](../../CONTRIBUTING.md).
