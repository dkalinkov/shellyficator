# Contributing

Bug reports, documentation fixes, and focused improvements are welcome. For a
larger feature, open an issue describing the use case and proposed behavior first.
Check the [roadmap](docs/wiki/roadmap.md) for planned work.

Use [the development guide](docs/wiki/development.md) to set up the project.
Describe the problem, resulting behavior, and validation in each pull request.
Keep changes focused and add regression coverage for behavioral fixes. Do not use
live devices or your regular database in automated tests.

Every pull request runs [CI](.github/workflows/ci.yml): Go formatting, `go vet`,
tests with the race detector, `govulncheck`, template formatting, a check that
vendored assets match `package-lock.json`, workflow linting, a container build
for `amd64` and `arm64`, and a release dry run. The `ci-ok` check must pass
before merging.

Before submitting:

- Run `make css`, `make test`, and `git diff --check`.
- Run `make format-check` when changing templates; use `gofmt` for Go changes.
- Update relevant wiki pages when behavior or configuration changes.
- Remove credentials, real device identifiers, household inventories, and private
  screenshots from examples and fixtures. Use synthetic values.

Bug reports should include the app version, installation method, device generation
and model, reproduction steps, and sanitized errors. Report sensitive security
issues as described in [SECURITY.md](SECURITY.md).

Contributions are provided under the project's [MIT license](LICENSE). Third-party
assets must retain their original license and attribution.
