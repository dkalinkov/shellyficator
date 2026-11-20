#!/bin/sh
# Usage: scripts/fuzz.sh [duration], for example scripts/fuzz.sh 2m
set -eu

duration=${1:-30s}

grep -rlE --include='*_test.go' '^func Fuzz[A-Za-z0-9_]+\(' internal | sort | while read -r file; do
  package=./$(dirname "$file")
  for target in $(sed -nE 's/^func (Fuzz[A-Za-z0-9_]+)\(.*/\1/p' "$file"); do
    echo "== $target in $package for $duration"
    go test "$package" -run='^$' -fuzz="^$target\$" -fuzztime="$duration"
  done
done
