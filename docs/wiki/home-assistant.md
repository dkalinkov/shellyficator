# Home Assistant

The experimental app supports Home Assistant OS and Supervised on amd64 and
arm64. For Container or Core, run the [standalone image](deployment.md#linux-container).

## Install

1. Add `https://github.com/dkalinkov/shellyficator` through the button or under
   **Settings → Apps → App store → ⋮ → Repositories**.

   [![Add the Shellyficator repository to your Home Assistant.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fdkalinkov%2Fshellyficator)

2. Install and start **Shellyficator**. Choose **Open Web UI** or **Show in sidebar**.

Home Assistant authenticates administrators. The app's standalone login, OIDC,
and PocketBase data admin are disabled.

## Settings

Set **Shared device password** for protected devices and restart the app.
Home Assistant saves it to [`/data/options.json`](configuration.md#options-file).

## How the app runs

- Image `ghcr.io/dkalinkov/shellyficator-ha` runs as UID 10001 after taking
  ownership of the data mount.
- Host networking supports mDNS and ARP. The app listens at `172.30.32.1:8090`
  and accepts HTTP from the supervisor at `172.30.32.2`.
- Backups stop the app to copy `/data` consistently. Enable **Watchdog** to
  restart an unresponsive app.

## Maintaining the app

After the [release](releases.md#publishing-a-release) publishes the image, open a
separate PR to bump `homeassistant/shellyficator/config.yaml` and copy its notes to
the app's `CHANGELOG.md`. Update `DOCS.md` when user-facing behavior changes.
CI verifies the exact image tag is public and covers every configured architecture.

To test a branch, append `#branch-name` to the repository URL. The configured
version still needs a published image.
