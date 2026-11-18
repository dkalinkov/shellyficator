#!/bin/sh
# Print the CHANGELOG.md section for one version, without its heading.
# Usage: scripts/release-notes.sh 0.4.0
set -eu

version=${1:?usage: release-notes.sh VERSION}
changelog=${2:-CHANGELOG.md}

notes=$(awk -v version="$version" '
  /^## / {
    if (found) exit
    heading = $0
    sub(/^## +/, "", heading)
    split(heading, words, " ")
    if (words[1] == version) { found = 1; next }
  }
  found { print }
' "$changelog")

# Strip leading and trailing blank lines.
notes=$(printf '%s\n' "$notes" | sed -e '/./,$!d')
if [ -z "$notes" ]; then
  printf 'No "## %s" section with content in %s\n' "$version" "$changelog" >&2
  exit 1
fi
printf '%s\n' "$notes"
