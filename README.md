# Shellyficator

Requires Go 1.27. Development and CSS builds also need Node.js 24.

## Run

```sh
make run
```

Open [localhost:8090](http://localhost:8090) and create an account. Data admin (`/_/`) starts with the same credentials but has a separate account.

Use `make dev` for automatic restarts. Refresh the browser after CSS or JavaScript changes.

## Build and test

```sh
make build
make test
./bin/shelly-manager serve --http=127.0.0.1:8090 --dir=./pb_data
```

## Configuration

- Preserve `pb_data/` across upgrades.
- Set `SHELLY_DEVICE_PASSWORD` for protected devices (username `admin`).
- Set `SHELLY_COOKIE_SECURE=true` behind an HTTPS reverse proxy.
- Pass variables through the environment; `.env` files are not loaded. See [.env.example](.env.example).
- For unattended setup, set `SHELLY_ADMIN_EMAIL` and `SHELLY_ADMIN_PASSWORD` (12+ characters), then run `go run . setup --dir=./pb_data`.
- Choose **Discover**. If devices are missing, use **Advanced discovery** with an IP, hostname, or private subnet such as `192.168.1.0/24`.

## Docker on Linux

```sh
docker build -t shelly-manager:local .
docker run --rm --name shelly-manager \
  --network=host \
  -v shelly-manager-data:/data \
  shelly-manager:local serve --http=127.0.0.1:8090 --dir=/data
```

Preserve the `shelly-manager-data` volume. Bind mounts need write access for UID/GID 10001. Change `--http` for LAN access or use a reverse proxy. Health check: `/healthz`.

Use `make run` for Docker Desktop development. With bridge networking, publish port 8090 and supply the LAN range under **Advanced discovery**.

[Scope](PRODUCT_BRIEF.md) · [Status and next steps](IMPLEMENTATION.md) · [Network research](research/network-discovery-2026-09-30.md)
