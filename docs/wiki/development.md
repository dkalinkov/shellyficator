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
| `make fuzz` | Run each fuzz target for 30 seconds (`FUZZTIME=2m` to change) |
| `make format-check` | Check HTML template formatting |
| `make format` | Format HTML templates |
| `make docker` | Build local container images |

On a clean checkout, run `make css` before Go tests because static assets are
embedded by Go. Generated `internal/web/static/app.css` is intentionally ignored.
Run `gofmt` on changed Go files, relevant tests, and `git diff --check`. For service
or concurrency changes, include the race detector. Broaden tests when the scope
warrants it; use fake clients and temporary databases, not live device writes.

## Continuous integration

[CI](../../.github/workflows/ci.yml) runs on every pull request and on `main`.
The `ci-ok` job summarizes it and is the required check for merging. Local
equivalents:

| CI step | Local command |
| --- | --- |
| Build assets; vendored files match the lockfile | `npm ci --ignore-scripts && npm run build && git diff --exit-code -- internal/web/static` |
| Template formatting | `make format-check` |
| Go formatting, tidy modules, vet | `gofmt -l .`, `go mod tidy -diff`, `go vet ./...` |
| Tests | `go test -race ./...` |
| Known vulnerabilities | `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` |
| Container image | `make docker` |
| Release dry run | `goreleaser release --snapshot --clean` |

Workflow files are checked with actionlint and zizmor. Actions are pinned to full
commit SHAs; Dependabot proposes updates for actions, Go modules, npm packages,
and base images weekly. After CI passes on `main`, the `edge` image is published.
See [releases](releases.md) for the release workflow.

## Fuzzing

Parsers and comparisons that handle device or network input have Go fuzz tests
(`func Fuzz…` in `_test.go` files). `go test` runs their saved inputs on every
pull request; the [Fuzz workflow](../../.github/workflows/fuzz.yml) explores new
inputs for a few minutes per target each week. To fuzz locally, run `make fuzz`
or one target:

```sh
go test ./internal/domain -run='^$' -fuzz='^FuzzDifferences$' -fuzztime=2m
```

A failure writes the input under `testdata/fuzz/<target>/`. Fix the bug and
commit that file so it stays a regression test. For a failure in the weekly
workflow, the log prints each input and its path.

## Security scans

Use a disposable checkout for scans so local dotenv files, databases, backups,
and credentials are never included. Keep reports in `test-results/`, which is
ignored by Git. Generate Go coverage before SonarQube analysis:

```sh
make css
mkdir -p test-results
go test -coverprofile=test-results/coverage.out ./...
```

Start a local SonarQube Community Build server, bound to loopback (port 19000
avoids common local services). Its embedded database is suitable for local scans:

```sh
docker run -d --name shellyficator-sonarqube \
  -p 127.0.0.1:19000:9000 \
  -e SONAR_ES_BOOTSTRAP_CHECKS_DISABLE=true sonarqube:community
```

Open `http://127.0.0.1:19000`, change the initial admin password, create the
`shellyficator` project, and generate an analysis token. Supply it through the
`SONAR_TOKEN` environment variable, then run the scanner using the repository's
`sonar-project.properties`:

```sh
docker run --rm --network host --user "$(id -u):$(id -g)" \
  -e SONAR_TOKEN -e SONAR_HOST_URL=http://127.0.0.1:19000 \
  -v "$PWD:/usr/src" sonarsource/sonar-scanner-cli:latest
```

Trivy checks dependencies (including development tools), Dockerfile settings,
and accidental secrets. Scan the built image separately to include its Go
toolchain and Alpine packages:

```sh
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD:/src:ro" -v "$PWD/test-results:/reports" \
  aquasec/trivy:0.76.2 --cache-dir /reports/trivy-cache fs \
  --scanners vuln,misconfig,secret --include-dev-deps --no-progress \
  --skip-dirs /src/.git --skip-dirs /src/node_modules \
  --skip-dirs /src/test-results --skip-dirs /src/.scannerwork \
  --format json --output /reports/trivy-filesystem.json /src
make docker
docker save -o test-results/shellyficator.tar shellyficator:local
docker run --rm --user "$(id -u):$(id -g)" \
  -v "$PWD/test-results:/reports" \
  aquasec/trivy:0.76.2 --cache-dir /reports/trivy-cache image \
  --input /reports/shellyficator.tar --scanners vuln,secret --no-progress \
  --format json --output /reports/trivy-image.json
docker rm -f shellyficator-sonarqube
```

Record scanner versions, image digests, database update timestamps, and remaining
findings with the results. A dependency with no published fix stays visible;
review its exposure rather than suppressing the advisory to make a scan pass.

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
