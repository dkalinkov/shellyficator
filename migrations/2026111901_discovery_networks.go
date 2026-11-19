package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection := core.NewBaseCollection("discovery_networks")
		collection.Fields.Add(&core.TextField{Name: "prefix", Required: true, Max: 18})
		collection.AddIndex("idx_discovery_networks_prefix", true, "prefix", "")
		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("discovery_networks")
		if err != nil {
			return err
		}
		return app.Delete(collection)
	})
}
