# Shellyficator scope

Manage Gen2+ devices from the homelab and detect configuration drift that breaks physical wall controls. The reported link to firmware updates is unverified.

Go, embedded PocketBase/SQLite, HTML templates, HTMX, Tailwind CSS, and daisyUI abyss. Assets are embedded; schema changes use Go migrations.

## First release

- Discovery, persistent device/channel inventory, rooms, tags, and notes.
- Status, device UI links, configuration snapshots, approved baselines, and differences.
- Manual switch/input editing with capability checks, capture before writes, and readback verification.
- Manual and daily issue scans with saved reports and configurable frequency, time, and timezone.
- Shared device password management and dashboard login.

See [implementation status](IMPLEMENTATION.md) for delivered features and remaining work.

## Data and configuration rules

- Validate hosts through `/shelly`. Persist reported ID/MAC; addresses, names, and firmware can change. Key channels by device and component ID.
- Rediscovery and failed reads preserve local metadata, history, and baselines. Show the last known comparison muted when current settings cannot be checked.
- Keep captures timestamped and immutable through the app. Baseline approval requires a fresh read and changes only on explicit approval. Data admin can modify all records.
- Compare per-channel input type, relay behaviour, enable/lock flags, power-loss state, and timers. Detached mode is valid; flag differences from the approved baseline.
- Distinguish changed, added, and removed settings. Snapshots omit secrets and may omit scripts/schedules; physical-control acceptance still requires a physical check.
- Status refresh runs every 30 seconds while visible, with pause/manual controls and shared completed results across viewers. Retry transient reads once after 350 ms; mark Offline only after three consecutive failed checks and at least 90 seconds without validated contact. Persist the failure streak; successful contact clears it. Identity mismatches are immediate errors. Read configuration on recovery, every five minutes, and on manual Refresh; retry unavailable settings on the next poll. Automatic reads update current settings without creating snapshots or approving baselines. Issue scans and a persisted refresh interval remain planned.
- Issue reports record start/completion times, per-device outcomes, and setting links. Check drift, reachability, disabled authentication, and reported faults; distinguish authentication failures from connectivity failures.
- Keep stale, missing, stateless, false, and zero readings distinct.

## Interface rules

- Compact searchable desktop rows; stacked entries on phones. Give secondary columns equal widths and centered content; keep cell padding and badge heights consistent. Pack visible status icons together without reserving space for hidden icons. Row clicks open details; name links retain keyboard and native browser behaviour.
- Separate IP and room columns. Keep the input count and chevron beside the device name; expand inputs beneath the row without navigating. Preserve filters, expansion, focus, and scroll through updates.
- Combine search and room filters with multiple colored tag filters. Match any selected tag; preserve filters through refreshes, edits, and URL reloads. Filtering and grouping query saved inventory without contacting devices or waiting for network reads; device refresh and discovery are separate workflows.
- Optionally group the filtered list by room, with room colors and device counts; show devices without a room last. Collapse rooms independently and remember their state in the browser tab. Preserve grouping through refreshes, edits, and URL reloads.
- Show input type/state, relay mode/state, enable/lock/invert flags, power-loss behaviour, and channel notes. Modes/settings come from the latest configuration read; physical state, relay state, and Wi-Fi come from status polling. Label the settings read time explicitly; failed configuration reads preserve saved settings and show an unavailable warning independently of live status.
- Use a single reachability/fault icon plus cloud connectivity in the Status column, with accessible labels and tooltips. During the offline grace period show a muted last-seen indication. Preserve saved relay/input values, Wi-Fi, cloud connections, and comparisons, with muted stale values and last-read timestamps. Show an amber restart icon only when current status reports `sys.restart_required`; hide it for missing, false, stale, or unavailable readings. Show a confirmed current cloud connection using mint and a reduced-motion-aware shine; retain its last reported connection muted after missed checks. Keep inactive cloud indicators subdued on device details. Show drift counts and unavailable comparisons as text. Omit power, voltage, current, and energy from the UI; keep temperature, uptime, firmware, MAC, and connection details on the device page/tooltips.
- Show device-reported firmware offers as compact icons in the inventory Status column and on device details. Use amber for stable offers, cyan for beta offers, red for failures, green for verified success, and muted saved readings; tooltips retain the offered version and read time. Keep desktop inventory rows at one line. Confirm single or selected-device installations in a dialog; the batch action respects current filters. Default to stable updates; beta installs require selection. Run updates serially with configuration captures before/after and unchanged baselines. Show progress and persistent results in the status icons. Verify the actual installed version for two minutes after reboot; never infer success from an accepted request or automatically repeat an update command.
- Colours: green reachable/matching; red offline/faults; amber drift/security gaps; blue models/input controls; violet detached mode. Each room/tag has a name-based color, shared by its labels and suggestions. Wi-Fi: green ≥ -60 dBm, amber ≥ -70, red below -70; mute stale readings.
- Tooltips support hover, focus, touch, and Escape. Respect reduced motion.
- Room/tag editing supports existing or new names, removable tag chips, explicit Save, and Cancel/Escape. Pause polling while editing; retain drafts on failure. See [editing rules](research/organization-editing-2026-09-30.md).
- Device pages separate overview, channels/physical inputs, configuration comparison/history, and notes. Refresh status reads one device without adding a snapshot; refresh/capture/approval preserve unfinished notes. See [device-page research and rules](research/device-details-2026-10-02.md). Planned Issues and Settings views hold reports and scheduling/credential controls. Use short labels; omit repeated explanations and tutorials.

## Credentials

Current shared credentials come from the environment. Planned stored credentials must be encrypted with a deployment-supplied key and excluded from snapshots, logs, and links.

Roll out password changes on one selected device first. Verify its UI, Shellyficator, and Home Assistant; update the corresponding Home Assistant credentials and track partial failures. Device authentication, HTTPS, and MQTT authentication are separate settings.

Device UI links preserve the saved scheme/port and open a new tab. Both the server and browser need device-network access.

## Later

Selected restoration, profiles, network/MQTT/script/schedule/webhook editing, Wi-Fi provisioning, repeatable notifications, individual passwords, and Home Assistant app packaging.

## References

- [Shelly API](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly/), [switches](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Switch/), [inputs](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Input/), [authentication](https://shelly-api-docs.shelly.cloud/gen2/General/Authentication/).
- [Home Assistant integration](https://www.home-assistant.io/integrations/shelly/), [network findings](research/network-discovery-2026-09-30.md).
