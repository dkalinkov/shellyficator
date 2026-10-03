package inventory

import (
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
)

func (s *Store) OrganizationOptions() (domain.OrganizationOptions, error) {
	devices, err := s.Devices()
	if err != nil {
		return domain.OrganizationOptions{}, err
	}
	return domain.Options(devices), nil
}

// Organization edits local categorization without replacing notes or readings.
func (s *Store) Organization(id, room, tags string) error {
	return s.saveMetadata(id, room, tags, nil)
}

func (s *Store) saveMetadata(id, room, tags string, notes *string) error {
	return s.App.RunInTransaction(func(app core.App) error {
		record, err := app.FindRecordById("devices", id)
		if err != nil {
			return deviceLookupError(err)
		}
		options, err := New(app).OrganizationOptions()
		if err != nil {
			return err
		}
		room, selected, err := domain.CanonicalOrganization(room, tags, options)
		if err != nil {
			return err
		}
		record.Set("room", room)
		record.Set("tags", strings.Join(selected, ", "))
		if notes != nil {
			record.Set("notes", strings.TrimSpace(*notes))
		}
		return app.Save(record)
	})
}
