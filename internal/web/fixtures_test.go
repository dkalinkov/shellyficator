package web

import (
	"encoding/json"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/devices"
	"shellyficator/internal/domain"
	"shellyficator/internal/inventory"
	"shellyficator/internal/shelly"
)

// Database fixtures stay in tests; the production server only knows services.
type testServer struct {
	*Server
	store   *inventory.Store
	client  *shelly.Client
	devices *devices.Service
}

func (s *testServer) record(id string) (*core.Record, error) {
	return s.store.App.FindRecordById("devices", id)
}

func (s *testServer) records() ([]*core.Record, error) {
	return s.store.App.FindRecordsByFilter("devices", "", "name,device_id", 0, 0)
}

func (s *testServer) channelNotes(id string) map[string]string {
	notes, err := s.store.ChannelNotes(id)
	if err != nil {
		panic(err)
	}
	return notes
}

func (s *testServer) baselineConfig(record *core.Record) map[string]any {
	baseline, err := s.store.Baseline(domainRecord(record))
	if err != nil {
		panic(err)
	}
	return baseline
}

func objectField(record *core.Record, name string) map[string]any {
	var result map[string]any
	_ = json.Unmarshal([]byte(record.GetString(name)), &result)
	return result
}

func formatStoredTime(value string) string {
	parsed, _ := time.Parse("2006-01-02 15:04:05.000Z", value)
	return formatTime(parsed)
}

// Unsaved record changes model regression fixtures independently of persistence.
func domainRecord(record *core.Record) domain.Device {
	return domain.Device{
		ID: record.Id, DeviceID: record.GetString("device_id"), Name: record.GetString("name"),
		Model: record.GetString("model"), MAC: record.GetString("mac"), Firmware: record.GetString("firmware"),
		Generation: record.GetInt("generation"), Address: record.GetString("address"), AuthEnabled: record.GetBool("auth_enabled"),
		Room: record.GetString("room"), Tags: domain.SplitTags(record.GetString("tags")), Notes: record.GetString("notes"),
		Reachable: record.GetBool("reachable"), StatusAvailable: record.GetBool("status_available"), ConfigAvailable: record.GetBool("config_available"),
		ConsecutiveFailures: record.GetInt("consecutive_failures"), IdentityMismatch: record.GetBool("identity_mismatch"), Problem: record.GetString("problem"),
		LastSeen: record.GetDateTime("last_seen").Time(), LastChecked: record.GetDateTime("last_checked").Time(),
		LastCaptured: record.GetDateTime("last_captured").Time(), LastStatusRead: record.GetDateTime("last_status_read").Time(),
		Status: objectField(record, "status"), Config: objectField(record, "config"), BaselineID: record.GetString("baseline"),
	}
}
