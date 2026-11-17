# Releases

[VERSION](../../VERSION) is the version source. Increment the patch for fixes and the minor for features; while the app is `0.x`, breaking changes also increment the minor. Record changes in [CHANGELOG.md](../../CHANGELOG.md).

Update those files, run `make css` and `make test`, commit, and create an annotated tag matching `VERSION`. Build from the clean tagged commit:

```sh
git tag -a "v$(cat VERSION)" -m "Shellyficator $(cat VERSION)"
make build
make docker
```

`make build` and `make docker` report the base version for a clean matching tag, otherwise append `-dev`. `--version` also reports the commit, dirty marker, and UTC build time; the UI shows the app version. `make dev`, plain Go builds, and Docker builds without metadata report a development version. Docker images use `shellyficator:local` and `shellyficator:<app-version>` tags.

Database schema changes use separate dated Go migrations. Preserve app data across upgrades.

See [deployment and backups](deployment.md) before upgrading an installation.
