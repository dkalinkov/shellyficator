package inventory

import (
	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
)

func (s *Store) SaveFirmwareUpdates(updates map[string]domain.FirmwareUpdate) error {
	return s.App.RunInTransaction(func(app core.App) error {
		for id, update := range updates {
			record, err := app.FindRecordById("devices", id)
			if err != nil {
				return deviceLookupError(err)
			}
			record.Set("firmware_update", update)
			if err := app.Save(record); err != nil {
				return err
			}
		}
		return nil
	})
}
