package inventory

import (
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/pocketbase/pocketbase/core"
)

func labelName(value string) string { return strings.Join(strings.Fields(value), " ") }

// SplitTags keeps multiword labels intact and accepts pasted comma/newline lists.
func SplitTags(value string) []string {
	var tags []string
	seen := map[string]bool{}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		tag := labelName(part)
		key := strings.ToLower(tag)
		if tag != "" && !seen[key] {
			seen[key] = true
			tags = append(tags, tag)
		}
	}
	return tags
}

func organizationOptions(records []*core.Record) (rooms, tags []string) {
	roomNames, tagNames := map[string]string{}, map[string]string{}
	for _, record := range records {
		if room := labelName(record.GetString("room")); room != "" {
			key := strings.ToLower(room)
			if _, found := roomNames[key]; !found {
				roomNames[key] = room
			}
		}
		for _, tag := range SplitTags(record.GetString("tags")) {
			key := strings.ToLower(tag)
			if _, found := tagNames[key]; !found {
				tagNames[key] = tag
			}
		}
	}
	for _, room := range roomNames {
		rooms = append(rooms, room)
	}
	for _, tag := range tagNames {
		tags = append(tags, tag)
	}
	for _, values := range [][]string{rooms, tags} {
		sort.Slice(values, func(i, j int) bool { return strings.ToLower(values[i]) < strings.ToLower(values[j]) })
	}
	return rooms, tags
}

func (s *Store) OrganizationOptions() (rooms, tags []string, err error) {
	records, err := s.Devices()
	if err != nil {
		return nil, nil, err
	}
	rooms, tags = organizationOptions(records)
	return rooms, tags, nil
}

// Organization edits local categorization without replacing notes or readings.
func (s *Store) Organization(id, room, tags string) error {
	return s.saveMetadata(id, room, tags, nil)
}

func (s *Store) saveMetadata(id, room, tags string, notes *string) error {
	return s.App.RunInTransaction(func(app core.App) error {
		record, err := app.FindRecordById("devices", id)
		if err != nil {
			return err
		}
		rooms, knownTags, err := New(app).OrganizationOptions()
		if err != nil {
			return err
		}
		canonical := func(value string, options []string) string {
			value = labelName(value)
			for _, option := range options {
				if strings.EqualFold(value, option) {
					return option
				}
			}
			return value
		}
		room = canonical(room, rooms)
		selected := SplitTags(tags)
		for i, tag := range selected {
			selected[i] = canonical(tag, knownTags)
		}
		tags = strings.Join(selected, ", ")
		if utf8.RuneCountInString(room) > 100 {
			return errors.New("room names can contain at most 100 characters")
		}
		if utf8.RuneCountInString(tags) > 500 {
			return errors.New("selected tags can contain at most 500 characters in total")
		}
		record.Set("room", room)
		record.Set("tags", tags)
		if notes != nil {
			record.Set("notes", strings.TrimSpace(*notes))
		}
		return app.Save(record)
	})
}
