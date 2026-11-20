# Usage and monitoring

## Inventory

Multiple selected tags match any selected tag. Filtering uses saved inventory.
Rooms, tags, and notes are local to Shellyficator.

## Refresh and stale readings

The visible inventory polls every 30 seconds. Concurrent viewers share completed
results. Manual refresh also reads configuration, as do recovery and the
five-minute configuration interval.

Offline status requires three consecutive failed checks and at least 90 seconds
without validated contact. Identity mismatches appear immediately. Failed reads
preserve previous values and timestamps. Status and configuration can go stale
independently, and a successful contact clears the failure streak.

## Captures and baselines

**Capture config** saves fresh settings. **Approve current config** saves a fresh
capture and makes it the baseline. Automatic reads only update current readings.

Review changes and test physical controls before replacing a baseline. History is
read-only in the dashboard. Captures cannot be restored to a device.

## Changes to watch

**Settings → Changes to watch** defaults to warnings for button behavior and
behavior after power loss. All other categories are recorded for review.
Device overrides replace the global policy until cleared. Changing the policy
leaves device settings and approved baselines intact.

## Coverage limits

Snapshots cover `Shelly.GetConfig`. Webhook actions, schedules, script source, and
external automation are outside that response. Known secret fields and URL
credentials are redacted, but private network details can remain. A baseline
comparison cannot verify physical controls or identify who changed a setting.
