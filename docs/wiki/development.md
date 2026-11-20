# Development

Use the Go version in `go.mod`, Node.js 24, and GNU Make 4.3+.

```sh
cp .env.example .env
make dev
```

`make dev` rebuilds and restarts with Air. Reload the browser after asset edits.

## Useful commands

| Command | Purpose |
| --- | --- |
| `make run` | Run on localhost |
| `make build` | Build `bin/shellyficator` |
| `make test` | Build assets, test, and vet |
| `go test -race ./...` | Check concurrency changes |
| `make format` / `make format-check` | Format or check HTML templates |
| `make docker` | Build `shellyficator:local` |
| `docker build --target homeassistant -t shellyficator-ha:local .` | Build the Home Assistant image |

Run `make css` before calling Go directly on a clean checkout. Go embeds
`internal/web/static/app.css`. Use fake clients and temporary databases in tests.

## Continuous integration

[`ci-ok`](../../.github/workflows/ci.yml) is required for merges. For release
checks and image tags, see [releases](releases.md).

## Fuzzing

`make fuzz` runs each target for 30 seconds. Override with `FUZZTIME=2m`.
To run a single target, use `go test` directly.

```sh
go test ./internal/domain -run='^$' -fuzz='^FuzzDifferences$' -fuzztime=2m
```

The [weekly workflow](../../.github/workflows/fuzz.yml) prints failing inputs.
Commit them under `testdata/fuzz/<target>/` as regression fixtures.

## Security scans

Scan in a disposable checkout and keep reports in `test-results/`.
Generate coverage for SonarQube.

```sh
make css
mkdir -p test-results
go test -coverprofile=test-results/coverage.out ./...
```

Start SonarQube on loopback.

```sh
docker run -d --name shellyficator-sonarqube \
  -p 127.0.0.1:19000:9000 \
  -e SONAR_ES_BOOTSTRAP_CHECKS_DISABLE=true sonarqube:community
```

At `http://127.0.0.1:19000`, change the admin password and create the
`shellyficator` project. Set its analysis token as `SONAR_TOKEN`.

```sh
docker run --rm --network host --user "$(id -u):$(id -g)" \
  -e SONAR_TOKEN -e SONAR_HOST_URL=http://127.0.0.1:19000 \
  -v "$PWD:/usr/src" sonarsource/sonar-scanner-cli:latest
```

Use Trivy on both source and image to cover development dependencies and runtime packages.

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

Record scanner versions, image digests, advisory database dates, and findings.
Keep advisories visible while reviewing exposure and available fixes.

## Making changes

Follow the [architecture boundaries](architecture.md). Add dated migrations for
schema changes and leave applied migrations intact.

Templates live in `internal/web/templates`, browser modules in
`internal/web/static`, and shared styles in `ui/styles.css`. Reuse daisyUI and
Tailwind controls. See the [interface guidelines](interface-guidelines.md).
