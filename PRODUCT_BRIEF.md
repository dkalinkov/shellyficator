# Shellyficator product brief

This app will provide one dashboard for administering Shelly devices on the home network. The owner currently uses Home Assistant for automation and visits each Shelly web interface to change device settings. The main reported problem is that some firmware updates change switch configuration, including changing input behaviour to detached, which interrupts physical wall controls. The cause of those changes has not been investigated here.

The first release will make configuration visible, preserve snapshots, compare current settings with a manually approved baseline, and support manual editing of basic settings. Most physical controls are momentary push buttons; a few are ordinary toggle switches. Each device and channel therefore needs its own configuration.

This brief records the brainstorming decisions from 2026-09-30. The first implementation covers mDNS discovery with a cached-neighbor fallback, explicit address/subnet scans, persistent inventory, status, notes, configuration captures, baselines, and comparisons. The complete first release below remains in progress; see [implementation status](IMPLEMENTATION.md). The [network discovery research](research/network-discovery-2026-09-30.md) records 16 identified devices and later application verification of the entire fleet. Identity and reachability are verified again on each discovery.

## Name and identity

The name chosen on 2026-10-01 is **Shellyficator**. The logo includes **9000** as a playful addition to the wordmark. The tagline is *Keeping your Shellys in line. Allegedly.*

## Deployment and technical direction

The application will run in the homelab. Its interface will primarily serve desktop use and remain comfortable for occasional phone use. Packaging as a Home Assistant app is a future goal.

The confirmed stack is Go with HTML templates and HTMX, styled with Tailwind CSS and daisyUI's abyss theme. PocketBase is embedded in the Go application, providing SQLite, application authentication, and a data administration interface in one service. Device discovery and communication belong in Go; PocketBase stores inventory, notes, snapshots, baselines, and eventually scan results. Versions are pinned and Go schema migrations are kept in the repository. CSS, JavaScript, and fonts are served locally from the compiled binary.

## First release

| Area | Agreed behaviour |
| --- | --- |
| Discovery | Find devices already connected to the network and allow manually supplied addresses. |
| Inventory | Keep physical devices distinct from their channels; preserve devices that cannot currently be reached. |
| Organisation | Assign a room to each device and optional tags. |
| Notes | Store editable notes on both physical devices and individual channels. |
| Status | Show connectivity, channel state, meaningful measurements, and reported health information where supported. |
| Configuration | Show current basic settings and support manual editing, prioritising switches and inputs. |
| History | Capture configuration snapshots and compare them with an approved baseline. |
| Baselines | Let the owner explicitly mark current configuration as correct. |
| Issue scans | Run daily by default and on demand, with configurable scheduling. |
| Security | Support one shared device password and authenticated device communication. |
| Device access | Provide an Open device UI link in each device row's action menu and on its detail page. |

## Device identity and local information

Use the reported device ID and MAC to identify a device; keep its address, name, and firmware as changing attributes. Validate candidate hosts through the Shelly identification endpoint. Normal discovery uses mDNS and the host's cached LAN neighbors, with unicast mDNS probes and a retry for transient identity connection failures. Bounded subnet scans and manual host entry are separate advanced actions. The workstation does not receive multicast browse replies; the neighbor fallback has now identified the 16-device fleet with successful status and configuration reads.

Associate channels with their physical device and reported component ID. Preserve rooms, tags, notes, history, and baseline selection across address changes and temporary failures. Keep names reported by the device visible alongside locally saved information.

Notes should be available next to settings and have a short preview in the inventory. They remain editable while the device is unreachable. A channel note might explain which physical button controls which light or why a particular input mode is required.

## Configuration snapshots and baselines

Maintain three distinct concepts:

- **Current configuration:** the latest successfully read settings, with a capture time.
- **Snapshot history:** immutable dated captures, associated with the device identity and installed firmware version.
- **Approved baseline:** a configuration explicitly marked as correct by the owner.

Capture settings during issue scans and before app initiated configuration changes. After editing a setting, read it back and show whether the requested value was observed. A fresh read should support the Mark current settings as correct action.

Scans and edits preserve the approved baseline until the owner explicitly changes it. A device with no approved baseline needs a distinct state. A failed read leaves the last successful capture available with its timestamp and makes the current comparison unavailable.

Show differences per channel using readable setting names and saved/current values. Prioritise physical input type, output behaviour, input enablement or locking where supported, state after power loss, and automatic on/off timers. Detached mode can be intentional; a difference from that device's baseline is the reason to flag it.

First release repairs use the manual settings editor. Selected restoration and saved configuration profiles are later features.

Snapshots capture readable configuration. Wi-Fi passwords are omitted by the device API, and scripts, schedules, and other resources may require separate collection. Describe capture coverage accurately rather than presenting a configuration snapshot as a complete recoverable backup. Distinguish added or removed settings from changed values when firmware exposes a different configuration schema.

## Status refresh and issue scans

These activities have separate controls:

| Activity | Timing | Purpose |
| --- | --- | --- |
| Status refresh | Proposed starting interval of 10 seconds while the dashboard is open; configurable or disabled | Read state and connectivity from known devices. Provide a manual refresh button. |
| Scan for issues | Daily by default, plus Scan now | Read configuration, compare baselines, check security and reported health, and save a report. |
| Discovery | Separate discovery action | Find and identify newly connected devices. |

Automatic issue scans need an enable/disable setting, frequency, time of day, and timezone. Scheduled and manual scans use the same checks. Multiple viewers should share status results so opening another browser does not multiply device polling.

Status should include reachable/unreachable, last successful response, IP address, Wi-Fi connection and signal where available, output state, input state where reported, uptime, installed firmware, restart required, and reported errors. Display power, energy, voltage, current, and temperature when supported. Also expose authentication state, access point state, and cloud/MQTT connection information where available.

Use timestamps to distinguish fresh data from older observations. Missing measurements and stateless push button inputs must remain distinguishable from zero readings or an off state. Configuration matching a baseline verifies settings; physical wall operation still requires a physical check.

Initial issue checks cover configuration differences, unreachable devices, disabled authentication, and reported device errors. Reports need a start/completion time, per device outcomes, and links to relevant settings. An authentication failure must be distinguishable from a connectivity failure. An unreachable device has an unknown current configuration comparison.

## Passwords and direct access

Start with one shared device password. Each selected Shelly gets that password; the application stores a shared credential and uses it automatically from the Go backend. Protect the dashboard with its own login. Store device credentials encrypted, with the encryption key supplied through deployment secrets, and exclude secrets from snapshots, logs, and repository files.

Opening the Shelly web interface requires authentication to that device. For the initial Gen2+ fleet, the device username is `admin`. Home Assistant continues connecting directly to each device and needs the shared password saved in the corresponding Shelly integration entry. Changing device credentials requires updating affected Home Assistant entries too.

Introduce authentication on one selected device first, then verify access through its own interface, this app, and Home Assistant before proceeding across the fleet. Track the outcome for each device so partial password changes remain visible and manageable.

The Open device UI action opens a new browser tab at the device's currently known address, preserving its actual HTTP/HTTPS scheme and port. The browser must be able to reach that address. Keep credentials out of the link.

Device authentication, HTTPS, and MQTT authentication are separate security settings. Discovery can still identify an authenticated device through `/shelly`; management requires credentials.

## Proposed interface

The audience is technically proficient. Use short labels and status messages. Omit promotional headings, repeated explanations, and tutorials from routine screens.

The desktop inventory uses a searchable table with single-line device rows: device name, model/generation, address/room, status, Wi-Fi signal, an input disclosure button, and saved configuration comparison. Wi-Fi uses a larger colored icon with the numeric dBm value. A dedicated input button expands details immediately below the device; expanding does not navigate and open sections survive status refreshes and filtering. Entire device rows open device details, with a real name link for keyboard and standard browser navigation. Secondary controls retain their own actions. On phones, reflow rows into compact entries and stack expanded input cards.

### Inventory presentation decisions

The initial fleet has 16 devices with one or two relay channels. Input behavior and baseline differences are the most relevant settings for the reported wall-switch problem. Detached mode is already used across this fleet; use a neutral violet label and highlight differences from an approved baseline in amber. Expanded input cards show names, physical type and state, associated relay mode/state, enabled/locked/inverted flags, behavior after power loss, and saved input/relay notes. Configurations and input modes are captured data; relay/input state, Wi-Fi, and connectivity come from status polling. Keep capture/check timestamps in the disclosure and tooltips. Missing or unavailable state uses a dash; explicitly stateless inputs remain “Not reported.” Power, voltage, current, and energy readings are omitted from the application UI. Temperature, uptime, firmware, MAC, and connection details remain available on the device page or in identity tooltips.

- Keep reachability, changed-setting counts, unavailable comparisons, and device faults visible as text. Changed inputs color their disclosure button amber and show the changed mode within the expanded section.
- Use green for reachable/matching states, red for offline/device faults, amber for changes/security gaps, blue for models and input controls, and distinct neutral colors for input modes. Wi-Fi display bands are green at -60 dBm or higher, amber from -70 to below -60, and red below -70; retained readings use muted color when current status is unavailable.
- Replace repeated security sentences and empty notes cells with familiar icons. Notes, model/firmware metadata, and capture/check timestamps use tooltips. Tooltips support pointer hover, keyboard focus, touch, and Escape dismissal; useful details remain available on the device page.
- Use compact, consistent single-line desktop rows, a hover/focus highlight, tabular Wi-Fi numbers, and a compact inventory summary. Preserve text selection and native link behavior when making the rest of a row clickable. Input disclosure motion respects reduced-motion preferences.

These decisions apply [Carbon's data-table guidance](https://v10.carbondesignsystem.com/components/data-table/usage/), [Carbon's guidance on nonessential tooltip content](https://carbondesignsystem.com/components/tooltip/usage/), [GOV.UK's numeric table alignment](https://design-system.service.gov.uk/components/table/), [NN/g's findings on icon ambiguity](https://www.nngroup.com/articles/icon-usability/), and [W3C's hover/focus requirements](https://www.w3.org/WAI/WCAG22/Understanding/content-on-hover-or-focus.html). Shelly documents [input modes and their physical-input behavior](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Switch/) and [physical input settings and stateless button readings](https://shelly-api-docs.shelly.cloud/gen2/0.14/ComponentsAndServices/Input/).

The device page presents status, channels and input settings, device/channel notes, configuration history, and baseline comparison. The Issues view collects findings across the fleet and links to the device and setting involved. Settings holds discovery ranges, the shared credential configuration, and scan/refresh scheduling.

Room and tag assignment is available directly from the inventory: click the room button or tag icon to open a compact editor. A searchable room picker accepts existing or new names and allows clearing the room. Tags use removable chips with existing-value suggestions and inline creation. Both support keyboard and touch operation; Save includes unfinished tag text, and Cancel/Escape discards the draft. Automatic inventory requests pause while the editor is open. Saving retains search/room filters, open inputs, notes, and device readings, and updates the room filter choices without navigating. The device page uses the same controls. Research and boundaries are recorded in [room and tag editing research](research/organization-editing-2026-09-30.md).

## Delivery order and acceptance

1. Verify visibility from the intended homelab host, then implement discovery and persistent identity. Rediscovery at another address must retain local information.
2. Build the responsive inventory and device status views, including timestamps, rooms, tags, notes, and device UI links.
3. Add configuration capture, immutable history, manually approved baselines, and readable comparisons. A controlled input mode change must appear on the correct channel without replacing its baseline.
4. Add manual switch/input editing with capability validation, capture before writes, and readback verification. Exercise both push button and toggle switch configurations on selected test devices.
5. Add manual and scheduled issue scans with reports. Verify that unreachable and authentication failures produce the correct states and preserve prior history.
6. Add shared password management and verify the selected device flow with Home Assistant. Verify that persisted data, rendered pages, and logs keep credentials protected.

The exact initial editable field set and background scan defaults should be checked against live device capabilities during implementation. The 10 second status refresh interval is a proposal, not a confirmed performance result.

## Later features

- Firmware updates, with configuration captures before and comparisons after each update.
- Selected restoration from a snapshot, followed by verification.
- Saved configuration profiles and comparison against those profiles.
- Broader editing of network settings, MQTT, scripts, schedules, and webhooks.
- Provisioning new devices onto Wi-Fi directly through the app.
- Telegram or similar notifications for new or unresolved issues, with configurable repeat behaviour.
- Individual device passwords and further security management.
- Home Assistant app packaging.

## Technical references

- [Shelly mDNS advertisements](https://shelly-api-docs.shelly.cloud/gen2/General/mDNS/).
- [Shelly identification and configuration API](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly/).
- [Shelly switch settings and status](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Switch/) and [input settings](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Input/).
- [Shelly authentication](https://shelly-api-docs.shelly.cloud/gen2/General/Authentication/) and [Wi-Fi configuration](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/WiFi/).
- [Home Assistant Shelly integration](https://www.home-assistant.io/integrations/shelly/) and [upstream authentication flow](https://github.com/home-assistant/core/blob/dev/homeassistant/components/shelly/config_flow.py). Confirm behaviour on the installed Home Assistant version during acceptance.
- [PocketBase Go integration](https://pocketbase.io/docs/go-overview/) and [HTMX documentation](https://htmx.org/docs/).
