# Usage and monitoring

## Inventory

Search and combine room/tag filters, optionally group results by room, and expand
a device's inputs. Multiple selected tags match any selected tag. Filtering uses
saved inventory; it does not start network discovery. Rooms, tags, and notes are
local metadata and do not change settings on the device.

Open a device for channel readings, connection information, firmware, configuration
comparison, capture history, and notes. Missing values are distinct from false,
zero, or stale readings.

## Refresh and stale readings

While visible, the inventory polls every 30 seconds; pause and manual refresh
controls are available. This is not an unattended scheduled monitoring service.
Concurrent viewers share completed refresh results. Transient reads get one retry
after 350 ms. Offline status requires three consecutive failed checks and at
least 90 seconds without validated contact; identity mismatches are immediate
errors.

Configuration is read on recovery, every five minutes, and on manual refresh.
Status and configuration have independent timestamps and availability. A failed
read preserves saved values, displayed as stale or unavailable. A later successful
contact clears the failure streak.

## Captures and baselines

**Capture config** saves a fresh configuration snapshot without changing the
baseline. **Approve current config** reads fresh settings and explicitly selects
that capture as the baseline. Automatic reads update current settings without
adding snapshots or approving anything. Captures are immutable through the app,
although a data administrator can modify stored records.

Differences distinguish added, removed, and changed settings. Review intentional
changes and test physical controls before approving a replacement baseline.
Snapshots cannot currently be restored to a device.

## Changes to watch

In **Settings → Changes to watch**, choose which categories produce warnings:

| Category | Initial behavior |
| --- | --- |
| Button behavior | Warn |
| Behavior after power loss | Warn |
| Timers | Record only |
| Script component settings | Record only |
| Connectivity and access | Record only |
| Other settings, including unknown fields | Record only |
| Device metadata | Record only |

Device pages link to optional overrides. Clearing an override restores the
app-wide preferences. Preferences persist across restarts; changing them neither
approves a baseline nor writes settings to a device. Record-only differences
remain available for review.

## Coverage limits

Monitoring covers `Shelly.GetConfig`, not webhook action definitions, schedules,
script source, or external automation. Known secret fields and URL credentials
are redacted, but snapshots may still contain private hostnames, SSIDs, and other
network details. A matching configuration does not verify physical wiring or
prove all button actions work. Reads cannot identify who changed a setting or why.
