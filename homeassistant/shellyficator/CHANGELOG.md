# Changelog

Release notes of the Shellyficator version each app version runs. Full notes and
downloads: <https://github.com/dkalinkov/shellyficator/releases>

## 0.5.0

- Publish a Home Assistant app image, `ghcr.io/dkalinkov/shellyficator-ha`, for `amd64` and `arm64`. It takes ownership of Home Assistant's root-owned `/data` mount, then runs unprivileged.
- Read settings from a JSON options file named by `SHELLY_OPTIONS_FILE`, such as the `options.json` that Home Assistant writes for apps.
- Attach each release's build provenance as a Sigstore bundle, so archives can be verified without GitHub's attestation API.
- Keep empty configuration keys from merging their settings into another path when comparing configurations.
