package main

import (
	"log"

	"shellyficator/internal/bootstrap"
)

func main() {
	config, err := bootstrap.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	app := bootstrap.New(config, bootstrap.BuildInfo{
		Version: appVersion(), Commit: commit, BuildDate: buildDate,
	})
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
