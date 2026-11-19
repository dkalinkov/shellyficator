package domain

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Run with: go test ./internal/domain -run=^$ -fuzz=^FuzzDifferences$

func FuzzDifferences(f *testing.F) {
	f.Add([]byte(`{"sys":{"device":{"name":"Hall"},"cfg_rev":3},"switch:0":{"id":0,"in_mode":"follow"}}`),
		[]byte(`{"sys":{"device":{"name":"Hallway"},"cfg_rev":4},"switch:0":{"id":0,"in_mode":"flip"},"input:0":{}}`))
	f.Add([]byte(`{"a":[1,{"b":2}],"c":null,"d":{}}`), []byte(`{"a":[1,{"b":3}],"d":{"e":false}}`))
	f.Fuzz(func(t *testing.T, left, right []byte) {
		var baseline, current map[string]any
		if json.Unmarshal(left, &baseline) != nil || json.Unmarshal(right, &current) != nil {
			return
		}
		// Paths join keys with " / ", so a key containing "/" can make two
		// settings share a path. Device configuration keys are identifiers such
		// as "switch:0" and never contain "/", so such inputs are out of scope.
		if keysContain(baseline, "/") || keysContain(current, "/") {
			return
		}
		if same := Differences(baseline, baseline); len(same) != 0 {
			t.Fatalf("a configuration differs from itself: %+v", same)
		}
		forward, backward := Differences(baseline, current), Differences(current, baseline)
		if len(forward) != len(backward) {
			t.Fatalf("%d differences forward, %d backward", len(forward), len(backward))
		}
		for i, difference := range forward {
			if i > 0 && forward[i-1].Path >= difference.Path {
				t.Fatalf("paths are not sorted and unique: %q then %q", forward[i-1].Path, difference.Path)
			}
			if !difference.Before.Present && !difference.After.Present {
				t.Fatalf("%q is reported without a value on either side", difference.Path)
			}
			reverse := backward[i]
			if reverse.Path != difference.Path || !reflect.DeepEqual(reverse.Before, difference.After) || !reflect.DeepEqual(reverse.After, difference.Before) {
				t.Fatalf("comparison is not symmetric: %+v and %+v", difference, reverse)
			}
		}
	})
}

// keysContain reports whether a key on a flattened path contains text.
// Arrays are compared as whole values, so their contents are not paths.
func keysContain(value any, text string) bool {
	object, _ := value.(map[string]any)
	for key, entry := range object {
		if strings.Contains(key, text) || keysContain(entry, text) {
			return true
		}
	}
	return false
}
