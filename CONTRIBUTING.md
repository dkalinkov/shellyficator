# Contributing

Discuss larger changes in an issue first. See the [roadmap](docs/wiki/roadmap.md)
and [development guide](docs/wiki/development.md).

- Keep pull requests focused. Describe the problem, change, and validation.
- Run `make test` and `git diff --check`. Format Go changes with `gofmt` and check
  template changes with `make format-check`. Merges require [`ci-ok`](.github/workflows/ci.yml).
- Cover behavioral fixes with regression tests. Use synthetic data, fake devices,
  and temporary databases.
- Update the relevant docs for behavior or configuration changes.

Bug reports should include the app version, install method, device model, and
sanitized reproduction steps. See [SECURITY.md](SECURITY.md) for private reports.

Contributions use the [MIT license](LICENSE). Retain third-party licenses and attribution.
