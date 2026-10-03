package domain

import (
	"encoding/json"
	"sort"
)

func Differences(baseline, current map[string]any) []Difference {
	before, after := map[string]any{}, map[string]any{}
	flatten("", baseline, before)
	flatten("", current, after)
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	var result []Difference
	for key := range keys {
		old, hadOld := before[key]
		now, hasNow := after[key]
		oldJSON, _ := json.Marshal(old)
		nowJSON, _ := json.Marshal(now)
		if hadOld && hasNow && string(oldJSON) == string(nowJSON) {
			continue
		}
		result = append(result, Difference{Path: key, Before: SettingValue{Present: hadOld, Value: old}, After: SettingValue{Present: hasNow, Value: now}})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

func flatten(prefix string, object map[string]any, result map[string]any) {
	for key, value := range object {
		path := key
		if prefix != "" {
			path = prefix + " / " + key
		}
		if nested, ok := value.(map[string]any); ok && len(nested) > 0 {
			flatten(path, nested, result)
			continue
		}
		result[path] = value
	}
}
