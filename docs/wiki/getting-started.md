# Getting started

## Run from source

Follow the [quick start](../../README.md#quick-start) on a machine that can reach
your devices. `make run` listens at `127.0.0.1:8090` and stores data in `pb_data/`.

## First login

Create an operator account with a password of at least 12 characters. First-run
setup also creates a separate PocketBase administrator with the same credentials
if none exists. Complete setup locally before exposing the app. The data admin
at `/_/` can edit all records.

For unattended setup, set `SHELLY_ADMIN_EMAIL` and `SHELLY_ADMIN_PASSWORD` first.

```sh
make css
go run . setup --dir=./pb_data
```

The setup command creates both accounts and refuses to overwrite existing ones.

## Add and review devices

1. Choose **Discover**. For devices it misses, try [Advanced discovery](discovery.md).
2. Open a device, check its settings and physical controls, then approve its
   current configuration as a baseline.
3. Choose which differences raise warnings under **Settings → Changes to watch**.

See [usage and monitoring](usage.md) for reads, captures, and baselines.
