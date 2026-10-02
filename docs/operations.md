# Configuration and releases

## Configuration

The app loads optional `.env.local` and `.env` files from the working directory for all commands. Precedence: existing environment (including empty values), then `.env.local`, then `.env`. Quotes, comments, and `export KEY=value` are supported. Missing files are allowed; invalid or unreadable files stop startup without logging their contents. These files are ignored by Git and excluded from Docker images.

See [`.env.example`](../.env.example) for available variables:

- `SHELLY_DEVICE_PASSWORD`: shared password for protected devices, username `admin`.
- `SHELLY_COOKIE_SECURE=true`: use behind an HTTPS reverse proxy.
- `SHELLY_ADMIN_EMAIL` and `SHELLY_ADMIN_PASSWORD` (12+ characters): initial accounts for `go run . setup --dir=./pb_data`. Data admin has a separate account with the same initial credentials.

Restart after configuration changes; `make dev` restarts automatically. Choose **Discover**; use **Advanced discovery** with an IP, hostname, or private subnet if devices are missing.

`make dev`, `make run`, and `make build` rebuild CSS and copy vendor assets only when their inputs change or outputs are missing. Generated `internal/web/static/app.css` is ignored by Git. Development builds omit timestamps by default to reuse cached Go builds; set `BUILD_DATE` to include one.

## Docker

The [README example](../README.md#docker-on-linux) passes `.env` through `--env-file`. Use unquoted `KEY=value` entries without `export`, or omit the option if there is no file. For full dotenv syntax and `.env.local` overrides, mount the files read-only under `/data`.

The container runs as UID/GID 10001; bind mounts must be writable by that user. Preserve the data volume and reuse it when upgrading an existing install. Change `--http` for LAN access or use a reverse proxy. Health check: `/healthz`. With bridge networking, publish port 8090 and supply the LAN range under **Advanced discovery**.

## Releases

[VERSION](../VERSION) is the version source. Increment the patch for fixes and the minor for features; while the app is `0.x`, breaking changes also increment the minor. Record changes in [CHANGELOG.md](../CHANGELOG.md).

Update those files, run `make test`, commit, and create an annotated tag matching `VERSION`. Build from the clean tagged commit:

```sh
git tag -a "v$(cat VERSION)" -m "Shellyficator $(cat VERSION)"
make build
make docker
```

`make build` and `make docker` report the base version for a clean matching tag, otherwise append `-dev`. `--version` also reports the commit, dirty marker, and UTC build time; the UI shows the app version. `make dev`, plain Go builds, and Docker builds without metadata report a development version. Docker images use `shellyficator:local` and `shellyficator:<app-version>` tags.

Database schema changes use separate dated Go migrations. Preserve app data across upgrades.
