package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		devices, err := app.FindCollectionByNameOrId("devices")
		if err != nil {
			return err
		}
		devices.Fields.Add(
			&core.NumberField{Name: "consecutive_failures", OnlyInt: true},
			&core.BoolField{Name: "identity_mismatch"},
			&core.DateField{Name: "last_status_read"},
		)
		if err := app.Save(devices); err != nil {
			return err
		}
		// Only successful legacy status reads have a known timestamp. For stale
		// copies, last_seen may refer to a newer identity-only response.
		_, err = app.DB().NewQuery("UPDATE devices SET last_status_read = last_seen WHERE status_available = TRUE AND status IS NOT NULL AND status != '' AND status != 'null'").Execute()
		return err
	}, func(app core.App) error {
		devices, err := app.FindCollectionByNameOrId("devices")
		if err != nil {
			return err
		}
		for _, name := range []string{"consecutive_failures", "identity_mismatch", "last_status_read"} {
			devices.Fields.RemoveByName(name)
		}
		return app.Save(devices)
	})
}
