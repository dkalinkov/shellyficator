# Implementation status

## Delivered

- Embedded PocketBase/SQLite, migrations, dashboard login, first-run setup, and data admin.
- Responsive inventory, input disclosures, device details, rooms/tags, and device/channel notes.
- Gen2+ discovery via IPv4 mDNS and cached LAN neighbors; explicit host/subnet scans under Advanced discovery. MAC identity and offline entries persist.
- Shared status polling, manual refresh, pause, configuration captures, baseline approval, and differences.
- Environment-supplied device credentials, embedded assets, automatic development restarts, and a non-root container with persistent data.

The app currently reads device settings; device writes are pending.

## Next

1. Verify discovery, status, captures, and Linux container host networking on the homelab host.
2. Add switch/input editing: capability checks, capture before writes, and readback. Test momentary and toggle controls physically.
3. Add issue scans, persistent reports, and daily scheduling with enable/frequency/time/timezone controls.
4. Add encrypted credentials and selected-device password changes; verify device UI and Home Assistant access.
5. Add persisted refresh intervals, access point/timer status, and settings filters.

Further scope: [product brief](PRODUCT_BRIEF.md#later).

## Recorded verification — 2026-09-30

- Go tests/vet, JavaScript syntax, CSS/native builds, and container build/readiness passed. Container ran as UID 10001.
- Tests cover drift, identity/address changes, stale/offline data, redaction, discovery bounds/retries, and metadata preservation.
- Temporary-database browser checks covered setup/login, CSRF, escaping, notes/baselines, room/tag editing, keyboard/touch use, and refresh/filter stability. No overflow at 320px/390px.
- WSL discovery checked 27 cached addresses and read all 16 devices in about 10.6 seconds; multicast browsing returned nothing. No device settings changed.
- Homelab networking and physical-control acceptance remain open. Docker Desktop host networking was not reachable from WSL.

Run instructions: [README](README.md).
