package domain

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// ConfigurationComparison separates watched differences from recorded-only
// differences without discarding either from snapshots or the raw comparison.
type ConfigurationComparison struct {
	Settings, Informational []Difference
}

func CompareConfiguration(baseline, current map[string]any, policies ...WatchPolicy) ConfigurationComparison {
	var policy WatchPolicy
	if len(policies) > 0 {
		policy = policies[0]
	}
	var result ConfigurationComparison
	for _, difference := range Differences(baseline, current) {
		if policy.Warn(ConfigurationGroup(difference.Path)) {
			result.Settings = append(result.Settings, difference)
		} else {
			result.Informational = append(result.Informational, difference)
		}
	}
	return result
}

func configurationMetadata(path string) bool {
	// Exact paths matter: a network MAC filter or a custom component's fw_id
	// setting must not be hidden by a blanket field-name exclusion.
	// https://shelly-api-docs.shelly.cloud/gen2/ComponentsAndServices/Sys/#configuration
	switch path {
	case "sys / cfg_rev", "sys / device / mac", "sys / device / fw_id":
		return true
	}
	// Component IDs identify the component rather than configure its behavior.
	parts := strings.Split(path, " / ")
	if len(parts) == 2 && parts[1] == "id" {
		component := strings.Split(parts[0], ":")
		if len(component) == 2 && component[0] != "" {
			id, err := strconv.Atoi(component[1])
			return err == nil && id >= 0
		}
	}
	// New or unknown fields are classified separately by ConfigurationGroup.
	return false
}

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

// flatten joins nested keys with " / ". The prefix already ends with the
// separator, so an empty key still adds a level instead of acting as the root.
func flatten(prefix string, object map[string]any, result map[string]any) {
	for key, value := range object {
		path := prefix + key
		if nested, ok := value.(map[string]any); ok && len(nested) > 0 {
			flatten(path+" / ", nested, result)
			continue
		}
		result[path] = value
	}
}
