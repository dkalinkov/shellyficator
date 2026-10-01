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

func versionDescription() string {
	details := []string{}
	if commit != "unknown" {
		details = append(details, "commit "+commit)
	}
	if buildDate != "unknown" {
		details = append(details, "built "+buildDate)
	}
	description := "Shellyficator v" + appVersion()
	if len(details) > 0 {
		description += " (" + strings.Join(details, ", ") + ")"
	}
	return description
}
