# Getting started

## Run from source

Use Go 1.27, Node.js 24, and GNU Make 4.3 or newer. Run the server on a machine
that can reach the devices' local HTTP interfaces.

```sh
git clone https://github.com/dkalinkov/shellyficator.git
cd shellyficator
cp .env.example .env
make run
```

Before starting, set `SHELLY_DEVICE_PASSWORD` in `.env` if your devices are protected
by a shared password. Dependencies and CSS are built by Make. The dashboard binds
to `127.0.0.1:8090`; data is stored in `pb_data/`.

## First login

Open <http://localhost:8090> and create an operator account. Passwords must contain
at least 12 characters. When no regular data administrator exists, setup also
creates a separate PocketBase administrator with the same initial credentials.
The data admin at `/_/` can edit all records; its account is distinct from the
ordinary dashboard login. Complete setup locally before exposing the service.

For unattended account creation, set `SHELLY_ADMIN_EMAIL` and
`SHELLY_ADMIN_PASSWORD`, then run this after `make css`:

```sh
go run . setup --dir=./pb_data
```

The CLI creates both accounts and refuses to overwrite an existing account with
the same email. Those variables do not automatically create accounts on `serve`.

## Add and review devices

1. Choose **Discover**. If no devices appear, use
   [Advanced discovery](discovery.md) with a host or private subnet.
2. Open a device and inspect its identity, channels, status, and settings.
3. Add rooms, tags, or notes to make the inventory easier to navigate.
4. Check that its configuration and physical controls behave as intended, then
   approve the current configuration. Approval reads and captures fresh settings.
5. Review **Settings → Changes to watch** to choose which differences matter.

A baseline is a comparison reference, not a restore point. Shellyficator does not
currently restore saved configuration or edit switch/input settings.

Next: [usage](usage.md), [configuration](configuration.md), and
[deployment](deployment.md).
