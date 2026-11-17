# Shellyficator

Requires Go 1.27 and GNU Make 4.3+. Development and CSS builds also need Node.js 24.

## Run

```sh
cp .env.example .env
make run
```

Edit `.env` as needed, then open [localhost:8090](http://localhost:8090) and create an account. Data admin: `/_/`.

Use `make dev` for automatic restarts. Refresh the browser after CSS or JavaScript changes.

## Build and test

```sh
make build
make test
./bin/shellyficator --version
./bin/shellyficator serve --http=127.0.0.1:8090 --dir=./pb_data
```

[VERSION](VERSION) holds the app version. See [configuration and releases](docs/operations.md) for dotenv precedence and the release workflow.

Configure **Settings → Changes to watch** to choose which differences from each
approved baseline deserve warnings. Button behavior and behavior after power loss
are watched by default. Timers, script component settings, connectivity/access,
other settings (including unknown fields), and device metadata default to Record
only. Full configuration snapshots and informational differences remain available.
Device pages link to optional overrides; clearing an override restores app-wide
preferences. Preferences persist across restarts and never approve a baseline or
change a device's settings.

Monitoring currently covers configuration returned by `Shelly.GetConfig`. Webhook
action definitions, schedules, script source, and external automations are not
captured. Password values are redacted. A matching watched configuration does not
prove that every button action works. Approve a new baseline after intentional
configuration changes; reads cannot determine who changed a setting or why.

## Docker on Linux

```sh
make docker
docker run --rm --name shellyficator \
  --env-file .env \
  --network=host \
  -v shellyficator-data:/data \
  shellyficator:local serve --http=127.0.0.1:8090 --dir=/data
```

Preserve `pb_data/` or the Docker data volume across upgrades; reuse the existing volume when upgrading an older install. Use `make run` for Docker Desktop development.

[Scope](PRODUCT_BRIEF.md) · [Status and next steps](IMPLEMENTATION.md) · [Architecture](docs/architecture-plan.md) · [Network research](research/network-discovery-2026-09-30.md)
