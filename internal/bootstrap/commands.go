package bootstrap

import (
	"errors"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/spf13/cobra"

	"shellyficator/internal/accounts"
	"shellyficator/internal/accountstore"
)

// BuildInfo is supplied by the executable, which owns embedded and linker values.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

func (build BuildInfo) description() string {
	details := []string{}
	if build.Commit != "" && build.Commit != "unknown" {
		details = append(details, "commit "+build.Commit)
	}
	if build.BuildDate != "" && build.BuildDate != "unknown" {
		details = append(details, "built "+build.BuildDate)
	}
	description := "Shellyficator v" + build.Version
	if len(details) > 0 {
		description += " (" + strings.Join(details, ", ") + ")"
	}
	return description
}

func registerCommands(app *pocketbase.PocketBase, config Config, build BuildInfo) {
	app.RootCmd.Use = "shellyficator"
	app.RootCmd.Short = "Shellyficator CLI"
	app.RootCmd.Version = build.Version
	app.RootCmd.SetVersionTemplate(build.description() + "\n")
	app.RootCmd.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Create the initial operator and data administrator from environment variables",
		RunE: func(_ *cobra.Command, _ []string) error {
			if config.AdminEmail == "" || len(config.AdminPassword) < 12 {
				return errors.New("set SHELLY_ADMIN_EMAIL and SHELLY_ADMIN_PASSWORD (at least 12 characters)")
			}
			if err := app.RunAllMigrations(); err != nil {
				return err
			}
			return accounts.New(accountstore.New(app)).Bootstrap(config.AdminEmail, config.AdminPassword)
		},
	})
}
