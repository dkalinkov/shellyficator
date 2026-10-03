package main

import (
	"errors"
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/spf13/cobra"

	"shellyficator/internal/accounts"
	"shellyficator/internal/accountstore"
	"shellyficator/internal/devices"
	"shellyficator/internal/inventory"
	"shellyficator/internal/shelly"
	"shellyficator/internal/web"
	_ "shellyficator/migrations"
)

func main() {
	if err := loadEnv(); err != nil {
		log.Fatal(err)
	}
	app := pocketbase.New()
	app.RootCmd.Use = "shellyficator"
	app.RootCmd.Short = "Shellyficator CLI"
	app.RootCmd.Version = appVersion()
	app.RootCmd.SetVersionTemplate(versionDescription() + "\n")
	app.RootCmd.AddCommand(&cobra.Command{Use: "setup", Short: "Create the initial operator and data administrator from environment variables", RunE: func(_ *cobra.Command, _ []string) error {
		email, password := os.Getenv("SHELLY_ADMIN_EMAIL"), os.Getenv("SHELLY_ADMIN_PASSWORD")
		if email == "" || len(password) < 12 {
			return errors.New("set SHELLY_ADMIN_EMAIL and SHELLY_ADMIN_PASSWORD (at least 12 characters)")
		}
		if err := app.RunAllMigrations(); err != nil {
			return err
		}
		return accounts.New(accountstore.New(app)).Bootstrap(email, password)
	}})
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		store := inventory.New(app)
		client := shelly.NewClient(os.Getenv("SHELLY_DEVICE_PASSWORD"))
		deviceService := devices.New(store, client, devices.Options{Discovery: shelly.NetworkDiscovery{}, Logger: app.Logger()})
		accountService := accounts.New(accountstore.New(app))
		if err := web.Register(e, deviceService, accountService, appVersion()); err != nil {
			deviceService.Close()
			return err
		}
		app.OnTerminate().BindFunc(func(event *core.TerminateEvent) error {
			deviceService.Close()
			return event.Next()
		})
		return e.Next()
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
