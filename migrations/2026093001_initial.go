package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		operators := core.NewAuthCollection("operators")
		if err := app.Save(operators); err != nil {
			return err
		}
		devices := core.NewBaseCollection("devices")
		for _, name := range []string{"device_id", "mac", "name", "model", "firmware", "address", "room", "tags", "notes", "problem", "baseline"} {
			devices.Fields.Add(&core.TextField{Name: name, Max: 8000})
		}
		devices.Fields.Add(&core.NumberField{Name: "generation"}, &core.BoolField{Name: "reachable"}, &core.BoolField{Name: "auth_enabled"}, &core.BoolField{Name: "status_available"}, &core.BoolField{Name: "config_available"}, &core.DateField{Name: "last_seen"}, &core.DateField{Name: "last_checked"}, &core.DateField{Name: "last_captured"}, &core.JSONField{Name: "status", MaxSize: 1048576}, &core.JSONField{Name: "config", MaxSize: 1048576})
		devices.AddIndex("idx_devices_mac", true, "mac", "")
		if err := app.Save(devices); err != nil {
			return err
		}
		snapshots := core.NewBaseCollection("snapshots")
		snapshots.Fields.Add(&core.RelationField{Name: "device", CollectionId: devices.Id, Required: true, CascadeDelete: true}, &core.JSONField{Name: "config", MaxSize: 1048576, Required: true}, &core.TextField{Name: "firmware"}, &core.TextField{Name: "reason"}, &core.AutodateField{Name: "captured", OnCreate: true})
		if err := app.Save(snapshots); err != nil {
			return err
		}
		channels := core.NewBaseCollection("channel_notes")
		channels.Fields.Add(&core.RelationField{Name: "device", CollectionId: devices.Id, Required: true, CascadeDelete: true}, &core.TextField{Name: "component", Required: true}, &core.TextField{Name: "notes", Max: 8000})
		channels.AddIndex("idx_channel_notes_device_component", true, "device, component", "")
		return app.Save(channels)
	}, func(app core.App) error {
		for _, name := range []string{"channel_notes", "snapshots", "devices", "operators"} {
			c, err := app.FindCollectionByNameOrId(name)
			if err != nil {
				return err
			}
			if err := app.Delete(c); err != nil {
				return err
			}
		}
		return nil
	})
}
