package migrations

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func TestRefreshHealthMigrationPreservesLegacyReadings(t *testing.T) {
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	if err := app.RunAllMigrations(); err != nil {
		t.Fatal(err)
	}
	var migration *core.Migration
	for _, item := range core.AppMigrations.Items() {
		if item.File == "2026100201_refresh_health.go" {
			migration = item
			break
		}
	}
	if migration == nil {
		t.Fatal("refresh migration not registered")
	}
	if err := app.RunInTransaction(migration.Down); err != nil {
		t.Fatal(err)
	}
	collection, err := app.FindCollectionByNameOrId("devices")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i, statusAvailable := range []bool{true, false} {
		record := core.NewRecord(collection)
		record.Set("mac", []string{"AABBCCDDEE01", "AABBCCDDEE02"}[i])
		record.Set("reachable", true)
		record.Set("status_available", statusAvailable)
		record.Set("status", map[string]any{"switch:0": map[string]any{"output": false}})
		record.Set("config", map[string]any{"switch:0": map[string]any{"in_mode": "momentary"}})
		record.Set("last_seen", time.Now().UTC().Add(-time.Minute))
		if err := app.Save(record); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, record.Id)
	}
	if err := app.RunInTransaction(migration.Up); err != nil {
		t.Fatal(err)
	}
	for i, id := range ids {
		record, err := app.FindRecordById("devices", id)
		if err != nil {
			t.Fatal(err)
		}
		if record.GetString("status") == "" || record.GetString("config") == "" || !record.GetBool("reachable") || record.GetInt("consecutive_failures") != 0 {
			t.Fatal("migration changed device readings or health")
		}
		if i == 0 && record.GetString("last_status_read") != record.GetString("last_seen") {
			t.Fatal("successful legacy status read lost its timestamp")
		}
		if i == 1 && record.GetString("last_status_read") != "" {
			t.Fatal("migration invented a timestamp for stale legacy status")
		}
	}
	if err := app.RunInTransaction(migration.Down); err != nil {
		t.Fatal(err)
	}
	collection, _ = app.FindCollectionByNameOrId("devices")
	if collection.Fields.GetByName("last_status_read") != nil {
		t.Fatal("rollback did not remove the health fields")
	}
	if count, err := app.CountRecords("devices"); err != nil || count != 2 {
		t.Fatal("rollback removed legacy devices")
	}
}
