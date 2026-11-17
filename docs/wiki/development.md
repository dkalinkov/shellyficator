# Development

Use the Go version in `go.mod`, Node.js 24, and GNU Make 4.3+. The lockfile pins
frontend dependencies; Make runs `npm ci --ignore-scripts` when needed.

```sh
cp .env.example .env
make dev
```

Air is pinned as a Go tool. Development watches source and asset inputs, rebuilds
embedded assets, and restarts the server. Refresh the browser after CSS/JavaScript
changes. Development uses the existing `pb_data/` directory, so run automated
integration checks against a temporary database instead.

## Useful commands

| Command | Purpose |
| --- | --- |
| `make css` | Build generated CSS and ensure vendored assets are present |
| `make run` | Build assets and run on localhost |
| `make dev` | Watch, rebuild, and restart |
| `make build` | Build `bin/shellyficator` with version metadata |
| `make test` | Go tests and vet |
| `go test -race ./...` | Check concurrency behavior |
| `make format-check` | Check HTML template formatting |
| `make format` | Format HTML templates |
| `make docker` | Build local container images |

On a clean checkout, run `make css` before Go tests because static assets are
embedded by Go. Generated `internal/web/static/app.css` is intentionally ignored.
Run `gofmt` on changed Go files, relevant tests, and `git diff --check`. For service
or concurrency changes, include the race detector. Broaden tests when the scope
warrants it; use fake clients and temporary databases, not live device writes.

## Making changes

Follow the [architecture boundaries](architecture.md). Put pure policy in the
domain, orchestration behind service interfaces, and wire implementations in
bootstrap. Keep presentation labels and formatting in web view builders.

Use a new dated Go migration for schema changes. Preserve existing user data;
do not rewrite a migration already used by an installation. Add regression tests
for behavioral changes, particularly identity, stale readings, baseline approval,
and firmware verification.

Templates live in `internal/web/templates`, browser modules and vendored assets
in `internal/web/static`, and shared styles in `ui/styles.css`. Use existing
daisyUI controls and Tailwind utilities. Follow the
[interface guidelines](interface-guidelines.md) for interaction and accessibility
rules. Keep third-party asset license files.

`make dev`, `make run`, and `make build` rebuild CSS and vendor assets when inputs
change or outputs are missing. Development builds omit timestamps to reuse Go
build caches; set `BUILD_DATE` to include one. Tailwind scans templates, browser
JavaScript, and status classes in `internal/web/templates.go` so dynamic controls
and status colors are included.

`VERSION` is the app version source. See [releases](releases.md)
for the changelog, tags, clean builds, and release metadata.
