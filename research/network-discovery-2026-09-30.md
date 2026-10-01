# Home network Shelly discovery — 2026-09-30

## Purpose and scope

Research for an app that discovers and eventually manages Shelly devices directly on a network. The investigation was read-only: discovery traffic and `GET /shelly` identity requests, with no relay, configuration, or firmware changes.

The observed LAN was `192.168.1.0/24`. The scanning workstation was WSL at `192.168.1.78` on `eth3`; the router was `192.168.1.1`, and Home Assistant was `192.168.1.137`. This inventory covers addresses found on that subnet during this investigation. It does not establish whether devices exist on other subnets, behind range extenders, or in other protocols.

## Result and evidence

- **16 distinct local IP devices** returned Shelly-compatible identity JSON from `GET /shelly`: 15 Shelly-branded devices and one Ogemray 25A Powered by Shelly device. The response was the evidence for manufacturer/ecosystem identity; router names alone were only candidate hints.
- The [Home Assistant screenshot](evidence/home-assistant-shelly-2026-09-30.png) shows **16 visible, collapsed top-level rows** and an integration header of **32 devices / 443 entities**. That header is a Home Assistant registry count, not proof of 32 distinct network hosts. Home Assistant [creates channel sub-devices for multi-relay Shellys](https://www.home-assistant.io/integrations/shelly/#entity-naming-generation-2); the exact makeup of this instance's 32 records was not inspected.
- The 16 local identities align by name with the 16 visible Home Assistant rows. The **Kitchen Ceiling Lights** to `kitchen.lights.main` match is inferred from the remaining row and local name; the screenshot does not show its underlying identifier.
- A later spot check on 2026-09-30 could reach the router and Home Assistant, but **none of the 16 prior Shelly IPs** answered `GET /shelly` from WSL. Several neighbor entries showed failed resolution. A native Windows request to `.31` also timed out. Current reachability therefore remains unresolved; the table below records the earlier successful identity responses, not a live status claim.

## Device inventory

Each IP below returned `/shelly` during the earlier scan. Display names come from the Home Assistant screenshot; device names and MACs come from the network identity results. IP addresses and display names can change, so a future app should re-identify a host before managing it.

| Home Assistant row | Device name / local identity | IP | Device family | MAC |
| --- | --- | --- | --- | --- |
| Bathroom Controls | `bathroom.controls.ventilator` | `192.168.1.52` | Mini 1PM Gen4 | `7C2C67642CEC` |
| Bathroom Lights | `bathroom.lights` | `192.168.1.31` | 2PM Gen4 | `7C2C677A0158` |
| Bedroom Bedside Controls Left | `bedroom.bedside.controls.left` | `192.168.1.43` | 2PM Gen4 | `7C2C677A021C` |
| Bedroom Bedside Controls Right | `bedroom.bedside.controls.right` | `192.168.1.183` | 2PM Gen4 | `7C2C6779E7A8` |
| Bedroom Door Controls | `bedroom.door.controls` | `192.168.1.113` | Mini 1PM Gen4 | `7C2C67642ABC` |
| Bedroom Lights | `bedroom.lights` | `192.168.1.224` | 2PM Gen4 | `7C2C677A011C` |
| Hallway Boiler | `hallway.boiler` | `192.168.1.240` | Ogemray 25A Gen3, Powered by Shelly | `B08184E1C56C` |
| Hallway Lights | `hallway.lights.main` | `192.168.1.101` | Mini 1PM Gen4 | `7C2C6763F3A8` |
| Kitchen Ceiling Lights (inferred match) | `kitchen.lights.main` | `192.168.1.247` | Mini 1PM Gen4 | `7C2C67642720` |
| Kitchen Controls | `kitchen.controls.sink` | `192.168.1.253` | Mini 1PM Gen4 | `7C2C67640364` |
| Living Room Lights | `living-room.lights` | `192.168.1.67` | 2PM Gen4 | `7C2C677A017C` |
| Living Room Mood Lights | `living-room.mood` | `192.168.1.222` | 2PM Gen3 | `34CDB0771460` |
| Living Room Picture Wall | `living-room.lights.picture-wall` | `192.168.1.225` | Plus 1PM Gen2 | `441793D541FC` |
| Small Bedroom Lights | `small-bedroom.lights` | `192.168.1.35` | 2PM Gen4 | `7C2C677A015C` |
| Terrace Lights | `terrace.lights.main` | `192.168.1.154` | Mini 1PM Gen4 | `7C2C6763FF74` |
| Wet Room | `wet-room` | `192.168.1.237` | 2PM Gen4 | `7C2C6779FF0C` |

The intermittently reachable `192.168.1.253` returned the raw device ID `shelly1pmminig4-7c2c67640364`, model `S4SW-001P8EU`, generation `4`, and app `Mini1PMG4`. Exact raw IDs and model codes for the other devices were not preserved in this note; obtain them again from `/shelly` before using them as app keys.

## What was tried

1. **mDNS browse:** Queried `_shelly._tcp.local` and `_http._tcp.local` from both WSL and native Windows. Neither multicast browse produced device advertisements during the test. Direct unicast mDNS queries did produce responses from `192.168.1.183` and `.240`, so the empty browse result does not establish that the devices lack mDNS. The reason multicast discovery failed from this workstation was not determined. Shelly [documents both service types](https://shelly-api-docs.shelly.cloud/gen2/General/mDNS/).
2. **Router DNS hints:** Reverse DNS yielded several obvious `shelly*` names and several custom room/control names. DNS names supplied candidate addresses but were insufficient to establish device identity.
3. **Fast HTTP sweep:** A first `/24` sweep used about 48 concurrent HTTP probes with a short connection timeout (~0.65 s) and returned zero identities. This was an inconclusive negative result.
4. **Slower address sweep and identity checks:** ICMP probes populated neighbor/ARP entries even where devices did not answer ping. Subsequent `GET /shelly` requests identified nine devices at first, then 15 after additional retries and address checks. `192.168.1.253` answered on later attempts, bringing the verified total to 16. The timing/neighbor-cache effect is an observation, not a proven root cause of every earlier miss.
5. **Full subnet follow-up:** A paced pass over the remaining addresses found no additional `/shelly` identities. It cannot prove that every device was online, reachable, or on this subnet at the time.
6. **Latest reachability check:** All 16 previous IPs failed from this WSL session while `192.168.1.1` and `.137:8123` responded. Do not silently remove a previously identified device after one failed scan.

The identification endpoint is documented by Shelly for [Gen2+](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly/#http-endpoint-shelly) and [Gen1](https://shelly-api-docs.shelly.cloud/gen1/#shelly). Its response includes generation and identity information needed to choose the correct API. The [Ogemray 25A](https://shelly-api-docs.shelly.cloud/gen2/Devices/PoweredByShelly/Ogemray25A/) is documented under Powered by Shelly, which is why a discovery app should identify by API response rather than by hostname prefix or brand label alone.

## Implications for the future app

- Use **several discovery paths**: mDNS when available, a bounded scan of user-approved local ranges, and manually supplied hosts. Validate each candidate with `/shelly`; do not treat ping, PTR names, or an open port as device proof.
- Use the device's reported ID/MAC as identity, and treat IP and friendly name as changing attributes. Keep device, channel/component, and Home Assistant entity counts distinct.
- Make scans tolerant of transient failures: modest concurrency, explicit connect/read timeouts, retries with a delay, and a state such as `last seen` rather than deleting a device immediately. Record why a candidate failed (no route, timeout, HTTP error, invalid JSON, unsupported generation).
- Read generation and capabilities before choosing management endpoints. Gen2+ exposes [RPC methods](https://shelly-api-docs.shelly.cloud/gen2/General/RPCChannels/) such as `Shelly.GetStatus`; Gen1 has a [different HTTP API](https://shelly-api-docs.shelly.cloud/gen1/). Do not infer available controls from a device's room name.
- Plan for secured devices. Home Assistant notes that some Gen2+ devices use [HTTPS with Enhanced Security](https://www.home-assistant.io/integrations/shelly/#shelly-enhanced-security); a plain HTTP scan is not a complete discovery strategy. Keep credentials outside the repository.
- Test discovery from the **actual deployment host and network segment**. This WSL workstation saw no multicast browse responses and later lost reachability to the previously verified Shelly IPs while retaining router/Home Assistant access. The app needs to report network visibility problems clearly.

## Open questions before implementation

1. Why did multicast browsing fail on both WSL and Windows, despite some unicast mDNS replies?
2. Why did the later session fail neighbor resolution for all known Shelly addresses while the router and Home Assistant remained reachable? Check AP/client isolation, routing, VLANs, and device availability from the intended app host.
3. What exactly accounts for Home Assistant's 32 device records? Inspect the expanded rows or registry to map channels/sub-devices to the 16 network identities.
4. Which devices require HTTPS/authentication, and what management operations should the first app version support? Verify from live `/shelly` and capability/status responses before designing controls.

## Later application verification on 2026-09-30

The application now combines multicast browsing with cached LAN neighbors and unicast mDNS probes. A browser-triggered discovery checked **27 cached addresses** and identified **all 16 devices**, with successful status and configuration reads for every device, in about **10.6 seconds**. The check used a temporary application database; all identities were read from the live devices. No switch settings, passwords, firmware, or network settings were changed.

Multicast browsing still returned no responses from this workstation, while directed mDNS queries returned device advertisements. Initial HTTP requests sometimes failed neighbor resolution, followed by successful requests after directed traffic or a retry. The specific multicast/neighbor-resolution cause remains unresolved; this observation explains why a multicast-only implementation was insufficient here. The earlier failure observations above remain historical, and homelab/container verification is still needed.

## Safe read-only verification example

```sh
curl --connect-timeout 2 --max-time 5 http://192.168.1.31/shelly
```

A failed request is an availability observation at that moment, not proof that the device does not exist.
