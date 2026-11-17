package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection := core.NewBaseCollection("watch_preferences")
		collection.Fields.Add(&core.TextField{Name: "scope", Required: true}, &core.JSONField{Name: "policy", Required: true, MaxSize: 4096})
		collection.AddIndex("idx_watch_preferences_scope", true, "scope", "")
		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("watch_preferences")
		if err != nil {
			return err
		}
		return app.Delete(collection)
	})
}
