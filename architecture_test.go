package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestApplicationLayerDependencies(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Dir(filename)
	allowed := map[string]map[string]bool{
		"main":         {"bootstrap": true},
		"bootstrap":    {"accountstore": true, "devices": true, "domain": true, "inventory": true, "shelly": true, "web": true, "oidc": true},
		"domain":       {},
		"devices":      {"domain": true},
		"inventory":    {"domain": true},
		"accountstore": {"domain": true},
		"shelly":       {"domain": true},
		"oidc":         {"domain": true},
		"web":          {"domain": true},
	}
	for layer, dependencies := range allowed {
		t.Run(layer, func(t *testing.T) {
			directory := filepath.Join(root, "internal", layer)
			if layer == "main" {
				directory = root
			}
			entries, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
					continue
				}
				file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, entry.Name()), nil, parser.ImportsOnly)
				if err != nil {
					t.Fatal(err)
				}
				for _, spec := range file.Imports {
					path, err := strconv.Unquote(spec.Path.Value)
					if err != nil {
						t.Fatal(err)
					}
					if internal, found := strings.CutPrefix(path, "shellyficator/internal/"); found {
						if !dependencies[internal] {
							t.Errorf("%s imports %s across its layer boundary", entry.Name(), path)
						}
						continue
					}
					if layer == "main" || layer == "domain" || layer == "devices" {
						first, _, _ := strings.Cut(path, "/")
						if strings.Contains(first, ".") || first == "shellyficator" {
							t.Errorf("%s imports non-standard dependency %s", entry.Name(), path)
						}
					}
				}
			}
		})
	}
}
