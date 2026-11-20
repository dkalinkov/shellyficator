# Deployment and backups

## Native binary

Download the Linux amd64 or arm64 archive, or run `make build`. Run under a
dedicated account with a writable data directory and access to the device network.
[Releases](https://github.com/dkalinkov/shellyficator/releases)

```sh
./bin/shellyficator serve --http=127.0.0.1:8090 --dir=./pb_data
```

## Linux container

Pin an exact tag from [Releases](https://github.com/dkalinkov/shellyficator/releases).
Use `make docker` and `shellyficator:local` for a local build.

```sh
docker run -d --name shellyficator --restart unless-stopped \
  --env-file .env \
  --network=host \
  -v shellyficator-data:/data \
  ghcr.io/dkalinkov/shellyficator:<version> serve --http=127.0.0.1:8090 --dir=/data
```

- Host networking supports LAN discovery on Linux. With bridge networking,
  publish port 8090 and use explicit discovery targets.
- Make bind mounts writable by UID/GID 10001. Mount dotenv files under `/data`
  for full parsing and `.env.local`. Docker's `--env-file` expects plain
  `KEY=value` entries.
- The example binds to loopback. The image default binds to `0.0.0.0:8090`.
  Docker checks HTTP readiness at `http://127.0.0.1:8090/healthz`. Adjust
  `--health-cmd` if you change the listen address or port.

## Remote access

Complete setup locally. For remote access, use an HTTPS proxy, set
`SHELLY_COOKIE_SECURE=true`, and restrict `/_/`. Operators share one inventory.

The dashboard and PocketBase APIs share a limit of ten password attempts per IP
over fifteen minutes. A successful sign-in resets it. Connections through the
same proxy share its IP for this limit.

## Backups and upgrades

1. Stop the app and back up the entire data directory or volume. Back up
   deployment secrets separately.
2. Start the intended version with the same data directory. Check `/healthz`,
   login, inventory, and capture history.
3. For a restore, stop the app and restore the full directory with its ownership.
   Verify it with the original version before upgrading.

Keep a pre-upgrade backup because a binary downgrade may not roll back migrations.
Use one instance per data directory. Protect backups as account and device data.
