# Shellyficator

*Keeping your Shellys in line. Allegedly.*

A local dashboard for Shelly Gen2+ devices: discover devices, see status, keep notes, and compare captured settings with a configuration you have marked as correct.

## Stack

| Layer | Choice |
| --- | --- |
| Server | Go 1.27 |
| Database, authentication, data admin | Embedded PocketBase 0.40.4 / SQLite |
| UI | Go HTML templates + HTMX 2.0.11 |
| Styling | Tailwind CSS 4.3.3 + daisyUI 5.7.47, **abyss** theme |
| Assets | Local IBM Plex Sans fonts, compiled CSS and JavaScript embedded in the binary |

## Run locally

```sh
make run
```

Open [http://localhost:8090](http://localhost:8090). On first use, create your dashboard account. The same initial email/password also opens **Data admin** (`/_/`); these are separate accounts after creation. Setup closes once the first dashboard account exists. Your Shelly device password is separate.

The included compiled assets allow a Go-only run. Go's automatic toolchain selection downloads Go 1.27 when required. Runtime data lives in `pb_data/`; preserve this directory across upgrades.

Choose **Discover**. The app listens for Shelly mDNS announcements for three seconds and also checks addresses in the host's existing LAN neighbor cache. It verifies candidates through `/shelly`, then reads status and configuration. Cached candidates receive a unicast mDNS query before HTTP verification, and transient identity connection failures get one retry. You do not need individual device IPs. Offline inventory entries are preserved; automatic discovery does not sweep a subnet.

The neighbor fallback reads `/proc/net/arp` on Linux and `arp.exe -a` on Windows. In WSL it also reads the Windows cache when interoperability is available. It accepts only private IPv4 hosts on eligible directly connected interfaces, with valid unicast MACs, and bounds each cache to 128 candidates. It does not change routes, firewall rules, or neighbor entries. A host with an empty cache and unavailable multicast needs an explicit scan to find new devices.

**Advanced discovery** provides separate buttons to scan a detected subnet, or accepts a device IP/hostname or a private IPv4 subnet such as `192.168.1.0/24`. Use it for another VLAN or hosts absent from both advertisements and the neighbor cache. A subnet scan checks each address and can take several minutes; it starts only when you choose it. The suggestions show up to four directly connected /24–/30 private networks, excluding loopback, VPN, and common container interfaces. Other ranges and IPv6-only devices need a manually supplied address/range for now. mDNS browsing currently uses IPv4.

The inventory refreshes status every 10 seconds while visible; the checkbox pauses it and **Refresh** requests a manual update. Configuration is captured separately: open a device and choose **Capture config**. Once you have checked its settings and physical controls, **Approve current config** performs another read and approves that capture as its baseline. Subsequent captures preserve that baseline and show differences.

This first slice reads device settings and saves local notes/baselines. Switch editing, scheduled issue scans, and setting device passwords are next steps; see [implementation status](IMPLEMENTATION.md).

### Read protected devices

Supply `SHELLY_DEVICE_PASSWORD` through your process environment or deployment secrets. The backend uses that shared password with username `admin`. It does not change passwords on devices. Credentials are excluded from configuration captures; the shared password is not stored in SQLite.

See [.env.example](.env.example) for available variables. The application does **not** automatically load `.env` files. Set `SHELLY_COOKIE_SECURE=true` when serving through an HTTPS reverse proxy. The machine running the application must be able to reach the device network; device UI links also require access from your browser.

For unattended initial setup, supply `SHELLY_ADMIN_EMAIL` and `SHELLY_ADMIN_PASSWORD` (at least 12 characters) as environment variables and run:

```sh
go run . setup --dir=./pb_data
```

### Build and verify

Node.js 24 is used for the CSS build; it is not needed at runtime.

```sh
make build
make test
./bin/shelly-manager serve --http=127.0.0.1:8090 --dir=./pb_data
```

### Development with automatic restarts

```sh
make dev
```

[Air](https://github.com/air-verse/air) is pinned as a Go tool in `go.mod`, so no global installation is needed. This command installs missing/outdated Node dependencies, then watches Go source, HTML templates, JavaScript, and CSS. Each change builds the UI assets, embeds them in a new binary, and restarts the app at [http://localhost:8090](http://localhost:8090) using the existing `pb_data/` directory. Changes to module/package files and the Makefile also trigger a rebuild.

Generated assets, binaries, database files, research, and test artifacts are excluded from the watcher. Changes are debounced; a failed build reports the error in the terminal and restarts the last successful binary. Fixing the source triggers another build. Temporary builds live in `.air/` and are removed when Air exits. Environment variables are inherited from the shell; `.env` files are not loaded automatically.

Refresh the browser to load changed CSS or JavaScript. Stop Air with Ctrl+C. `make run` remains available for a Go-only run without watching. Do not run both on port 8090 at the same time.

## Container

```sh
docker build -t shelly-manager:local .
docker volume create shelly-manager-data
docker run --rm --name shelly-manager \
  --network=host \
  -v shelly-manager-data:/data \
  -e TZ=Europe/Sofia \
  shelly-manager:local serve --http=127.0.0.1:8090 --dir=/data
```

This example is for Docker Engine running directly on a Linux homelab host. Host networking lets automatic discovery see that host's LAN and multicast traffic. It binds the dashboard to localhost; configure the listen address or a reverse proxy for homelab access. With Docker bridge networking, use a published port instead and Advanced discovery to specify the LAN range; the container's own subnet is not the device network. The container network must route to the device subnet.

Docker Desktop has different host networking behaviour: it is an opt-in feature and containers do not have direct access to the computer's physical interfaces. See [Docker's host networking documentation](https://docs.docker.com/engine/network/drivers/host/). For workstation development, `make run` directly in the host environment is the simpler discovery path.

The container runs as UID/GID 10001. For a bind mount, give that user write access to the data directory. `/healthz` provides an HTTP readiness check.

## Project notes

- [Product brief and first release scope](PRODUCT_BRIEF.md)
- [Implementation status and remaining work](IMPLEMENTATION.md)
- [Home network discovery and device inventory](research/network-discovery-2026-09-30.md)
- [Home Assistant evidence screenshot](research/evidence/home-assistant-shelly-2026-09-30.png)

Research addresses and identifiers are historical observations. Discovery verifies identity again, and refresh/capture checks the stored MAC before requesting protected device APIs. No historical devices are preloaded into the database.
