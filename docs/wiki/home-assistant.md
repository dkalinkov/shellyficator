# Home Assistant

Shellyficator runs as a Home Assistant app on Home Assistant OS and Supervised
installations, on `amd64` and `arm64`. Home Assistant Container and Core
installations cannot run apps; run the [container image](deployment.md#linux-container)
next to Home Assistant instead.

The app is experimental. It opens in its own browser tab on port 8090 and uses
Shellyficator's own login; it does not appear in the Home Assistant sidebar yet.

## Install

1. Add this repository to Home Assistant:

   [![Add the Shellyficator repository to your Home Assistant.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https%3A%2F%2Fgithub.com%2Fdkalinkov%2Fshellyficator)

   Or open **Settings → Apps**, go to the app store, and add
   `https://github.com/dkalinkov/shellyficator` under **⋮ → Repositories**.
2. Install **Shellyficator** from the store and start it.
3. Choose **Open Web UI** right away and create your account, as in
   [first login](getting-started.md#first-login). Until an account exists, anyone
   who can reach port 8090 on the Home Assistant host can create it.

## Settings

| Option | Purpose |
| --- | --- |
| Shared device password | Same as [`SHELLY_DEVICE_PASSWORD`](configuration.md) |

Restart the app after changing settings. Home Assistant writes them to
`/data/options.json`, which the app reads at startup; see
[options file](configuration.md#options-file).

## How the app runs

- **Image:** `ghcr.io/dkalinkov/shellyficator-ha`, built from the Dockerfile's
  `homeassistant` target with each release. It starts as root to take ownership
  of the `/data` mount, then runs as UID 10001.
- **Network:** host networking, so discovery can use mDNS and the ARP cache. The
  app listens on port 8090 on every interface of the Home Assistant host.
- **Data:** the app's `/data` folder holds the database and is included in Home
  Assistant backups. Backups are cold: the app stops while they run.
- **Health:** Home Assistant's watchdog restarts the app when `/healthz` stops
  answering.

## Maintaining the app

`repository.yaml` at the repository root makes this repository a Home Assistant
app repository; the app itself is in [`homeassistant/shellyficator/`](../../homeassistant/shellyficator).
Home Assistant reads `main` and offers an update as soon as `version` in its
`config.yaml` changes, so the app is updated after each release, never with it:

1. [Release](releases.md#publishing-a-release) as usual. Release pull requests
   do not change `homeassistant/`.
2. Once the release is published, open a pull request that sets `version` in
   `config.yaml` to the new version and adds the release notes to the app's
   `CHANGELOG.md` under `## <version>`. `scripts/release-notes.sh <version>`
   prints them.
3. CI's **Home Assistant app** check fails unless the image for that version is
   published for every architecture in `arch`, can be pulled without logging in,
   and the app's changelog has a section for it.

`DOCS.md`, shown in the app's Documentation tab, is also read from `main`, so it
should describe the released version. Update it in the same pull request.

To try a branch on a Home Assistant install before merging, add the repository
URL with the branch appended, for example
`https://github.com/dkalinkov/shellyficator#feat/my-change`. The app's `version`
must still name a published image.
