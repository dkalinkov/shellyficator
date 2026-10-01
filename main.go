package main

import (
	"errors"
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"

	"shelly-manager/internal/inventory"
	"shelly-manager/internal/shelly"
	"shelly-manager/internal/web"
	_ "shelly-manager/migrations"
)

func main() {
	app := pocketbase.New()
	app.RootCmd.AddCommand(&cobra.Command{Use: "setup", Short: "Create the initial operator and data administrator from environment variables", RunE: func(_ *cobra.Command, _ []string) error {
		email, password := os.Getenv("SHELLY_ADMIN_EMAIL"), os.Getenv("SHELLY_ADMIN_PASSWORD")
		if email == "" || len(password) < 12 {
			return errors.New("set SHELLY_ADMIN_EMAIL and SHELLY_ADMIN_PASSWORD (at least 12 characters)")
		}
		if err := app.RunAllMigrations(); err != nil {
			return err
		}
		return app.RunInTransaction(func(tx core.App) error {
			for _, name := range []string{"operators", core.CollectionNameSuperusers} {
				if _, err := tx.FindAuthRecordByEmail(name, email); err == nil {
					return errors.New("account already exists; use the data admin interface to manage it")
				}
				collection, err := tx.FindCollectionByNameOrId(name)
				if err != nil {
					return err
				}
				record := core.NewRecord(collection)
				record.SetEmail(email)
				record.SetPassword(password)
				record.SetVerified(true)
				if err := tx.Save(record); err != nil {
					return err
				}
			}
			return nil
		})
	}})
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		store := inventory.New(app)
		client := shelly.NewClient(os.Getenv("SHELLY_DEVICE_PASSWORD"))
		if err := web.Register(e, store, client); err != nil {
			return err
		}
		return e.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
