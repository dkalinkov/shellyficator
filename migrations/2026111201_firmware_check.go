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
		devices.Fields.Add(&core.JSONField{Name: "firmware_check", MaxSize: 65536})
		return app.Save(devices)
	}, func(app core.App) error {
		devices, err := app.FindCollectionByNameOrId("devices")
		if err != nil {
			return err
		}
		devices.Fields.RemoveByName("firmware_check")
		return app.Save(devices)
	})
}
