# Device details

Research and implementation direction, 2 October 2026.

## References and lessons

| Product / source | Observed pattern | Shellyficator application |
| --- | --- | --- |
| [Shelly Plus 1PM local interface](https://kb.shelly.cloud/knowledge-base/shelly-plus-1pm-web-interface-guide), [Gen4 guide](https://kb.shelly.cloud/knowledge-base/shelly-1pm-gen4-web-interface-guide) | Device identity and connection indicators in the header; outputs and inputs on Home; channel settings, connectivity and device settings have separate destinations. The UI adapts to the device's capabilities. | Put identity and connection state first. Separate outputs from physical inputs and show their relationship. Keep advanced device changes behind Open device UI. |
| [Home Assistant entity dialogs](https://www.home-assistant.io/blog/2023/03/01/release-20233/) | Entity state and relevant controls take priority; history and settings are available through secondary actions. An assumed state has a distinct presentation. | Make channel states easy to scan. Use explicit unknown/unavailable states. Move note editing into disclosure controls instead of repeating textareas everywhere. |
| [Home Assistant 2026.7 activity timeline](https://www.home-assistant.io/blog/2026/07/01/release-20267/#activity-logbook) | Chronological rail, timestamps, compact mobile presentation, consistent state language and less repetition of identity already established by the page. | Present configuration captures as a compact timeline, identify the approved baseline, omit repeated device names. Captures are not an activity log or telemetry history. |
| [Homey quick actions](https://support.homey.app/hc/en-us/articles/29337357359772-Use-quick-actions-on-your-devices), [capabilities](https://apps.developer.homey.app/the-basics/devices/capabilities), [settings](https://apps.developer.homey.app/the-basics/devices/settings) | Controls depend on actual capabilities; full controls and settings are separate. A sensor without a control does not need an action button. | Render the components actually reported by each device. State badges should look like readings; omit unsupported action buttons. |
| [Homey Insights](https://support.homey.app/hc/en-us/articles/360015370454-Track-your-home-with-Homey-Insights) | History is an explicit destination based on collected data and selectable metrics/timeframes. | Keep configuration capture history clearly named. There is no telemetry history store, so no invented charts. |
| [UniFi Connect device management](https://help.ui.com/hc/en-us/articles/25676801827479-Integrating-Lutron-Processors-with-UniFi-Connect), [offline diagnostics](https://help.ui.com/hc/en-us/articles/29703168513687-Troubleshooting-UniFi-Access-Device-Offline-Issue) | Device overview, settings and automations have separate purposes. Uptime, address and exact timestamps support diagnosis. | Distinct overview, channel, configuration and note sections; compact connection/identity facts and explicit last-checked/last-seen times. |
| [Carbon data tables](https://www.carbondesignsystem.com/building-blocks/core/components/data-table/guidelines) | Align comparable values; use progressive disclosure for supplementary detail and a dedicated destination for tasks that outgrow a row. | Keep the before/after settings comparison tabular, with contained scrolling. Hide channel note editors until needed. |

The layout decisions below are our synthesis of these references, not features attributed to every product. Reviewed the official reference screenshots for Shelly's local interface, Home Assistant's state dialogs/timeline and Homey's device tiles.

## Existing page audit

- Identity, reachability, firmware, network readings and password status were present but had weak hierarchy.
- Every component was rendered as a generic card; a physical input and its relay were not visibly related.
- Unknown/false values were shown as raw values; old readings could resemble current state.
- Channel note textareas occupied space even when nobody was editing.
- Metadata fields occupied the entire sidebar; firmware, MAC and connection facts deserved their own compact group.
- Configuration capture, approval, comparison and history existed, but freshness and reference state needed clearer treatment.
- Inventory already owns tested input mapping, mode colors and failure semantics. The detail view should reuse those interpretations.

## Resulting page contract

1. Identity header: breadcrumb, device name, model/generation, room/tags, connectivity, Refresh status and Open device UI.
2. Same-page navigation: Overview, Channels & inputs, Configuration, Notes. Native anchors keep all sections available without JavaScript and preserve existing note links.
3. Overview: compact Wi-Fi, uptime and password summaries. Connection facts lead with Shelly Cloud, then Wi-Fi/network, MQTT, restart warnings and last-seen time when it differs from the status check. Identity facts include device ID, firmware, MAC and address; the header owns model/generation, with the raw model code in its tooltip.
4. Outputs own relay state, temperature and power-loss settings. Physical inputs own type, mode, physical state and enabled/locked/inverted settings; their relay links jump to the output card. Unmapped relays retain their input behavior/lock settings. Other reported components retain a destination. Notes use per-channel disclosure editors.
5. Configuration owns comparison status and the path/baseline/latest table, Capture config, a short approval explanation and capture timeline. Status-check and settings-read timestamps appear together at the channel heading.
6. Notes: existing room/tag picker and device note; every channel note retains its exact anchor and persistence route.

## Redundancy pass

- Show each reading, setting and timestamp once in its owning section. Keep historical firmware on captures because it identifies the captured version.
- Header location, model and tags establish identity; organization fields remain available for editing.
- Configuration status appears in Configuration; remove duplicate summary/navigation counts and paragraphs that restate the status badge.
- Channel cards link to note editors; note excerpts appear only in the editor summaries. Preserve accessible textarea labels without repeating channel names visibly inside the editor.
- Remove repeated section kickers, summary explanations, healthy restart flags and empty annotation sections. Keep failure states, settings freshness and the meaning of baseline approval explicit.

## Data and interaction rules

- Follow PRODUCT_BRIEF.md: no power, voltage, current or energy readings; no device-control RPCs or new firmware/config editing features.
- Status-read availability and configuration-read availability are independent. Offline or failed-status values are unavailable; saved settings remain readable and visibly labeled.
- Physical input false means Off; null means Not reported. A detached input is a normal violet mode unless its configuration changed relative to the approved baseline.
- Input-to-relay mappings come from the existing inventory presenter, including explicit input_id. Missing mapping is not relay 0.
- Refresh status reads only the selected device and creates no snapshot. Capture config records a snapshot; approving a baseline captures and saves the reference. These operations do not change device settings.
- Treat elapsed uptime as a duration, not a decimal count of seconds. Zero is valid. Missing readings do not imply connected, healthy, or off.
- Keep read time, capture time and last-seen time distinct. This page shows the latest saved reading; it does not imply a continuous connection.
- Refresh/capture/approval replace only the displayed state, comparison and history through HTMX; metadata and channel-note drafts remain untouched. All POST forms retain CSRF tokens and native submission. Responsive layout stacks; long identities, names, notes and diff values wrap, while the comparison table has its own scroll container.

## Verification

Check online, offline, status failure, config failure, no baseline, missing data, multi-input mappings and reported faults; render all states without template errors. Verify the selected-device refresh preserves snapshot history/baseline and checks identity. Check desktop and 320/390px layouts, note deep links/disclosures, room/tag pickers, form saves and browser console errors.

Verification completed: `make test` (Go tests and vet), `make build`, JavaScript syntax checks and `git diff --check` passed. Browser checks covered the six reading/baseline states at desktop and mobile widths, with no document overflow at 320/390/768/1024/1440px, no browser errors, working note deep links and native behavior with JavaScript disabled. Simulated browser action responses preserved unsaved device/channel notes and disclosure state; submitted note/metadata forms included CSRF and the intended fields. Backend tests verified persistence, identity mismatch rejection and unchanged baseline/snapshot history during status refresh. Browser checks used isolated fixtures and intercepted action responses, not changes to household devices.

The redundancy pass passed Go tests/vet and the embedded build. Added coverage for mapped/unmapped setting ownership, unchanged inventory presentation, temperature aliases/zero readings and distinct last-seen timestamps. Browser checks covered matching, changed, unavailable and missing baselines, offline/status/config failures, empty data and faults at desktop/phone widths. Relay/note links worked, refresh preserved unfinished notes, and the application page reported no browser errors or horizontal overflow.

## Connection status

The Connection panel and inventory use the same cloud presentation. A valid reported `connected: true` is green Connected and takes precedence over saved settings. A reported false is amber Disconnected; an available `cloud.enable: false` makes it neutral Disabled. Failed status reads or offline devices show Unavailable without reusing a cached green connection. Missing or invalid connection data is Unknown. Failed configuration reads do not hide valid status or infer Disabled from stale settings. This distinction follows the independent configuration and status fields in the [Shelly Cloud component API](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Cloud/).

Cloud appears as a compact icon in the inventory's action area and links to the device's Connection panel. State and read time appear in its hover/focus tooltip and accessible name. Connected icons use brighter mint green, a subtle halo and a light traveling along the cloud outline every 2.4 seconds. Reduced motion removes the moving highlight. Disabled, unknown and unavailable icons stay static at 30% opacity, returning to full opacity on hover or keyboard focus. Inventory rows retain their original single-line height and update through existing manual and 30-second refreshes. Panel fields have icons and state-colored badges; SSID is informational blue, while restart requirements are amber. Wi-Fi states follow the [Wi-Fi component status API](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/WiFi/). The cloud row remains visible even without reported data so its unknown state is explicit.

Connection changes passed `make test`, `make build` and `git diff --check`. Tests cover status/config availability, cached values, missing/null/malformed flags and consistency between list and details. Browser checks covered eight connection/read states plus the inventory at 320/390/768/1024/1101/1440px without document or cloud-label overflow. Intercepted HTMX responses changed both cloud badges from green to amber, preserved an unfinished device note and retained working input disclosures; the application checks logged no browser errors. Previews used an isolated fixture database.

The icon-only correction passed the cloud/inventory tests and browser checks from 320px to 1440px. Desktop rows measured 52px again. Only connected cloud icons animated; their highlight changed over the cycle, reduced-motion mode disabled it, and keyboard focus exposed the connection tooltip. The icon stayed inside the action area at every checked width without document overflow or browser errors.

The stronger shine and contrast pass verified that the moving SVG highlight advances between ordinary browser frames, inactive icons use 30% opacity and keyboard focus restores full opacity. Reduced-motion mode hides the moving highlight. The development server serves the updated CSS; 320/390/768/1101/1440px layouts retain their fit.

The detail Connection panel shares the inventory cloud icon, highlight animation and mint color token. Its connected badge uses the same mint; inactive cloud icons and badges use 30% opacity, restoring full opacity when the row is hovered or its status is focused. The Shelly Cloud label stays readable. Browser checks covered connected, disconnected, disabled, unknown, offline and failed-status states at 320/390/768/1440px, including moving-highlight, focus and reduced-motion behavior. Go tests/vet, the embedded build and whitespace checks passed.
