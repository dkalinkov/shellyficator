# LAN discovery — 2026-09-30

Network: `192.168.1.0/24`; WSL workstation: `192.168.1.78`/`eth3`; router: `.1`; Home Assistant: `.137`.

`GET /shelly` identified 16 hosts: 15 Shelly devices and one Ogemray 25A. The [Home Assistant screenshot](evidence/home-assistant-shelly-2026-09-30.png) has 16 top-level rows and a 32-device/443-entity registry count. Channel sub-devices can explain extra records; the exact registry mapping was not inspected.

## Historical inventory

Addresses may change. Revalidate ID/MAC before management. Home Assistant names came from the screenshot; local names and MACs came from identity responses. The Kitchen Ceiling Lights match is inferred.

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

Only `.253`'s raw ID/model were retained: `shelly1pmminig4-7c2c67640364`, `S4SW-001P8EU`, Gen4, `Mini1PMG4`. Read other IDs/models again before using them as keys.

## Discovery findings

| Check | Result |
| --- | --- |
| Multicast mDNS, WSL and Windows | No browse replies for `_shelly._tcp.local` or `_http._tcp.local`; unicast replies from `.183` and `.240`. |
| Fast HTTP sweep | ~48 workers and ~0.65 s connect timeout found nothing. |
| Paced scan and retries | Identified nine, then 15, then 16 hosts; no additional identities in the remaining addresses. |
| Later reachability check | All 16 failed from WSL while router/Home Assistant responded; Windows also timed out on `.31`. |
| Application follow-up | Cached neighbors, unicast mDNS, and retry identified all 16 with status/config reads: 27 candidates, ~10.6 s. |

The application check used a temporary database and changed no device settings. Multicast browsing still returned nothing. The cause of intermittent neighbor resolution and missing multicast remains unresolved.

## Implementation constraints

- Combine mDNS, cached neighbors, explicit bounded subnet scans, and manual hosts. Validate with `/shelly`; DNS, ping, and open ports are only hints.
- Persist ID/MAC and last-seen timestamps. Preserve inventory after failures; report timeout, routing, HTTP, identity, and authentication failures separately.
- Use bounded concurrency, timeouts, and retries. Empty browse results or a fast sweep cannot establish absence.
- Select APIs by generation/capabilities: Gen2+ RPC, Gen1 HTTP. Account for HTTPS/authentication; keep credentials outside Git.
- Verify from the actual deployment host. Homelab/container networking, HTTPS coverage, and Home Assistant's registry mapping remain open.

```sh
curl --connect-timeout 2 --max-time 5 http://192.168.1.31/shelly
```

## References

- [mDNS](https://shelly-api-docs.shelly.cloud/gen2/General/mDNS/), [Gen2+ identity](https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Shelly/#http-endpoint-shelly), [RPC](https://shelly-api-docs.shelly.cloud/gen2/General/RPCChannels/), [Gen1](https://shelly-api-docs.shelly.cloud/gen1/#shelly).
- [Ogemray 25A](https://shelly-api-docs.shelly.cloud/gen2/Devices/PoweredByShelly/Ogemray25A/), [Home Assistant channel naming](https://www.home-assistant.io/integrations/shelly/#entity-naming-generation-2), [HTTPS](https://www.home-assistant.io/integrations/shelly/#shelly-enhanced-security).
