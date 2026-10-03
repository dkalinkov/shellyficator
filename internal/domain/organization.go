package domain

import (
	"errors"
	"sort"
	"strings"
	"unicode/utf8"
)

func LabelName(value string) string { return strings.Join(strings.Fields(value), " ") }

func SplitTags(value string) []string {
	var tags []string
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		tag := LabelName(part)
		key := strings.ToLower(tag)
		if tag != "" && !seen[key] {
			tags = append(tags, tag)
			seen[key] = true
		}
	}
	return tags
}

func Options(devices []Device) OrganizationOptions {
	rooms, tags := map[string]string{}, map[string]string{}
	for _, device := range devices {
		if room := LabelName(device.Room); room != "" {
			key := strings.ToLower(room)
			if _, found := rooms[key]; !found {
				rooms[key] = room
			}
		}
		for _, tag := range device.Tags {
			key := strings.ToLower(tag)
			if _, found := tags[key]; !found {
				tags[key] = tag
			}
		}
	}
	var result OrganizationOptions
	for _, value := range rooms {
		result.Rooms = append(result.Rooms, value)
	}
	for _, value := range tags {
		result.Tags = append(result.Tags, value)
	}
	for _, values := range [][]string{result.Rooms, result.Tags} {
		sort.Slice(values, func(i, j int) bool { return strings.ToLower(values[i]) < strings.ToLower(values[j]) })
	}
	return result
}

func CanonicalOrganization(room, tags string, options OrganizationOptions) (string, []string, error) {
	canonical := func(value string, known []string) string {
		value = LabelName(value)
		for _, option := range known {
			if strings.EqualFold(value, option) {
				return option
			}
		}
		return value
	}
	room = canonical(room, options.Rooms)
	selected := SplitTags(tags)
	for i, tag := range selected {
		selected[i] = canonical(tag, options.Tags)
	}
	if utf8.RuneCountInString(room) > 100 {
		return "", nil, errors.New("room names can contain at most 100 characters")
	}
	if utf8.RuneCountInString(strings.Join(selected, ", ")) > 500 {
		return "", nil, errors.New("selected tags can contain at most 500 characters in total")
	}
	return room, selected, nil
}
