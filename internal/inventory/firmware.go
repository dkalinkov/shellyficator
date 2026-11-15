package inventory

import (
	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
)

// SaveFirmwareUpdates admits a whole batch atomically. Later progress writes use
// the same operation with one device, preserving settings and baseline selection.
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
