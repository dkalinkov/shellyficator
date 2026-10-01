#!/bin/sh
set -eu

version=$(cat VERSION)
git_commit=$(git rev-parse --short=12 HEAD 2>/dev/null || printf 'unknown')
git_status=$(git status --porcelain --untracked-files=normal 2>/dev/null || true)
release_tag=$(git describe --exact-match --tags --match "v$version" HEAD 2>/dev/null || true)

build_type=${BUILD_TYPE:-dev}
if [ -z "${BUILD_TYPE:-}" ] && [ "$release_tag" = "v$version" ] && [ -z "$git_status" ]; then
  build_type=release
fi
case "$build_type" in
  dev|release) ;;
  *) printf 'Invalid BUILD_TYPE: %s\n' "$build_type" >&2; exit 1 ;;
esac

build_commit=${BUILD_COMMIT:-$git_commit}
if [ -z "${BUILD_COMMIT:-}" ] && [ -n "$git_status" ]; then
  build_commit="$build_commit-dirty"
fi
build_date=${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}
app_version=$version
if [ "$build_type" = dev ]; then
  app_version="$version-dev"
fi

output=${1:-bin/shellyficator}
if [ "$output" = docker ]; then
  docker build \
    --build-arg "BUILD_TYPE=$build_type" \
    --build-arg "BUILD_COMMIT=$build_commit" \
    --build-arg "BUILD_DATE=$build_date" \
    -t "shellyficator:$app_version" -t shellyficator:local .
else
  mkdir -p "$(dirname "$output")"
  CGO_ENABLED=0 go build -trimpath \
    -ldflags "-X main.buildType=$build_type -X main.commit=$build_commit -X main.buildDate=$build_date" \
    -o "$output" .
fi
