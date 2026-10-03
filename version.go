package main

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var baseVersion string

// Populated by scripts/build.sh. Plain go run builds remain development builds.
var (
	buildType = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func appVersion() string {
	version := strings.TrimSpace(baseVersion)
	if buildType != "release" {
		version += "-dev"
	}
	return version
}
