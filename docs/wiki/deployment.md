# Deployment and backups

## Native binary

Download a Linux `amd64` or `arm64` archive from
[Releases](https://github.com/dkalinkov/shellyficator/releases), or build with
`make build`, then run:

```sh
./bin/shellyficator serve --http=127.0.0.1:8090 --dir=./pb_data
```

Assets are embedded; Node.js is unnecessary at runtime. Run under a dedicated
service account with a persistent writable data directory and access to the
local device network. `/healthz` reports HTTP readiness, not device health.

## Linux container

Release images for `linux/amd64` and `linux/arm64` are published to
`ghcr.io/dkalinkov/shellyficator`. Pin an exact version so upgrades are
deliberate; see [container image tags](releases.md#container-image-tags).
Replace `<version>` with the newest number on the
[Releases](https://github.com/dkalinkov/shellyficator/releases) page:

```sh
docker run -d --name shellyficator --restart unless-stopped \
  --env-file .env \
  --network=host \
  -v shellyficator-data:/data \
  ghcr.io/dkalinkov/shellyficator:<version> serve --http=127.0.0.1:8090 --dir=/data
```

To build the image yourself, run `make docker` and use `shellyficator:local`
instead. You can [verify](releases.md#verifying-a-release) that a published image
was built by this repository's release workflow.

The image runs as UID/GID 10001. Bind mounts must be writable by that user.
The example listens only on loopback. The image's default command instead listens
on `0.0.0.0:8090`; choose exposure deliberately, especially with host networking.
For bridge networking, publish port 8090 and use explicit LAN discovery targets.
The Docker healthcheck polls `http://127.0.0.1:8090/healthz`. If you change the
listen address or port, set Docker's `--health-cmd` to the matching health URL.

Docker `--env-file` expects unquoted `KEY=value` entries without `export`. To use
the app's full dotenv parser and `.env.local` precedence, mount the files read-only
under `/data`, the container working directory.

## Remote access

Complete first-run setup locally. Place the service behind an HTTPS reverse proxy
for remote browser access and set `SHELLY_COOKIE_SECURE=true`. Restrict access to
trusted operators, including the powerful `/_/` data-admin interface. Each operator
can manage the shared inventory; per-user device permissions are not implemented.
Password sign-ins share a limit of ten attempts per client IP over fifteen
minutes across the dashboard and PocketBase authentication APIs, including the
data admin. A successful sign-in clears that client's attempts. Connections
through a reverse proxy share its IP for this limit.

## Backups and upgrades

1. Stop the running application/container so SQLite files are consistent.
2. Copy the entire data directory or volume to a protected backup destination.
3. Back up dotenv files or deployment secrets separately; keep both out of Git.
4. Pull or build the intended version, then start it using the same data directory/volume.
5. Check `/healthz`, login, inventory, and capture history before resuming use.

For a restore, stop the app and restore the complete directory with the required
ownership. Verify restoration with the version that created the backup before
upgrading. Do not point two application instances at the same data directory.

Go migrations evolve storage on startup. A binary downgrade is not a guaranteed
schema rollback; keep a pre-upgrade backup. The data directory contains account
data and private device/network information even though known snapshot secrets
are redacted.

See [release procedures](releases.md) for versioning and build tags.
