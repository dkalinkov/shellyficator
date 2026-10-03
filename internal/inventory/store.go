package inventory

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
)

// Store maps PocketBase records to domain types and keeps related writes atomic.
type Store struct{ App core.App }

func New(app core.App) *Store { return &Store{App: app} }

func object(record *core.Record, name string) map[string]any {
	var result map[string]any
	_ = json.Unmarshal([]byte(record.GetString(name)), &result)
	return result
}

func deviceFromRecord(record *core.Record) domain.Device {
	return domain.Device{
		ID: record.Id, DeviceID: record.GetString("device_id"), MAC: record.GetString("mac"),
		Name: record.GetString("name"), Model: record.GetString("model"),
		Generation: record.GetInt("generation"), Firmware: record.GetString("firmware"),
		Address: record.GetString("address"), AuthEnabled: record.GetBool("auth_enabled"),
		Room: record.GetString("room"), Tags: domain.SplitTags(record.GetString("tags")), Notes: record.GetString("notes"),
		Reachable: record.GetBool("reachable"), StatusAvailable: record.GetBool("status_available"), ConfigAvailable: record.GetBool("config_available"),
		ConsecutiveFailures: record.GetInt("consecutive_failures"), IdentityMismatch: record.GetBool("identity_mismatch"), Problem: record.GetString("problem"),
		LastSeen: record.GetDateTime("last_seen").Time(), LastChecked: record.GetDateTime("last_checked").Time(),
		LastCaptured: record.GetDateTime("last_captured").Time(), LastStatusRead: record.GetDateTime("last_status_read").Time(),
		Status: object(record, "status"), Config: object(record, "config"), BaselineID: record.GetString("baseline"),
	}
}

// writeDeviceState leaves local metadata and baseline selection untouched.
func writeDeviceState(record *core.Record, device domain.Device) {
	for key, value := range map[string]any{
		"device_id": device.DeviceID, "mac": device.MAC, "name": device.Name, "model": device.Model,
		"generation": device.Generation, "firmware": device.Firmware, "address": device.Address, "auth_enabled": device.AuthEnabled,
		"reachable": device.Reachable, "status_available": device.StatusAvailable, "config_available": device.ConfigAvailable,
		"consecutive_failures": device.ConsecutiveFailures, "identity_mismatch": device.IdentityMismatch, "problem": device.Problem,
		"last_seen": device.LastSeen, "last_checked": device.LastChecked, "last_captured": device.LastCaptured, "last_status_read": device.LastStatusRead,
		"status": device.Status, "config": device.Config,
	} {
		record.Set(key, value)
	}
}

func deviceLookupError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func (s *Store) Devices() ([]domain.Device, error) {
	records, err := s.App.FindRecordsByFilter("devices", "", "name,device_id", 0, 0)
	if err != nil {
		return nil, err
	}
	devices := make([]domain.Device, 0, len(records))
	for _, record := range records {
		devices = append(devices, deviceFromRecord(record))
	}
	return devices, nil
}

func (s *Store) Device(id string) (domain.Device, error) {
	record, err := s.App.FindRecordById("devices", id)
	if err != nil {
		return domain.Device{}, deviceLookupError(err)
	}
	return deviceFromRecord(record), nil
}

func (s *Store) SaveReading(reading domain.Reading, capture bool) (domain.Device, error) {
	return s.saveReading(reading, capture, false)
}

// ApproveReading selects its own fresh snapshot in the same transaction.
func (s *Store) ApproveReading(reading domain.Reading) (domain.Device, error) {
	if reading.Config == nil {
		return domain.Device{}, errors.New("capture configuration before approving a baseline")
	}
	return s.saveReading(reading, true, true)
}

func (s *Store) saveReading(reading domain.Reading, capture, approve bool) (domain.Device, error) {
	var saved domain.Device
	err := s.App.RunInTransaction(func(app core.App) error {
		record, err := app.FindFirstRecordByFilter("devices", "mac = {:mac}", dbx.Params{"mac": reading.Identity.MAC})
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			collection, err := app.FindCollectionByNameOrId("devices")
			if err != nil {
				return err
			}
			record = core.NewRecord(collection)
		}
		device := deviceFromRecord(record).ApplyReading(reading, capture, time.Now().UTC())
		writeDeviceState(record, device)
		if err := app.Save(record); err != nil {
			return err
		}
		if capture && reading.Config != nil {
			collection, err := app.FindCollectionByNameOrId("snapshots")
			if err != nil {
				return err
			}
			snapshot := core.NewRecord(collection)
			snapshot.Set("device", record.Id)
			snapshot.Set("config", reading.Config)
			snapshot.Set("firmware", reading.Identity.Version)
			snapshot.Set("reason", "Manual capture")
			if err := app.Save(snapshot); err != nil {
				return err
			}
			if approve {
				record.Set("baseline", snapshot.Id)
				if err := app.Save(record); err != nil {
					return err
				}
			}
		}
		saved = deviceFromRecord(record)
		return nil
	})
	return saved, err
}

func (s *Store) Failed(id, problem string, identityMismatch bool) error {
	return s.App.RunInTransaction(func(app core.App) error {
		record, err := app.FindRecordById("devices", id)
		if err != nil {
			return deviceLookupError(err)
		}
		writeDeviceState(record, deviceFromRecord(record).FailedCheck(problem, identityMismatch, time.Now().UTC()))
		return app.Save(record)
	})
}

func (s *Store) Metadata(id, room, tags, notes string) error {
	return s.saveMetadata(id, room, tags, &notes)
}

func (s *Store) Snapshots(id string) ([]domain.Snapshot, error) {
	records, err := s.App.FindRecordsByFilter("snapshots", "device = {:id}", "-captured", 50, 0, dbx.Params{"id": id})
	if err != nil {
		return nil, err
	}
	snapshots := make([]domain.Snapshot, 0, len(records))
	for _, record := range records {
		snapshots = append(snapshots, domain.Snapshot{
			ID: record.Id, DeviceID: record.GetString("device"), Firmware: record.GetString("firmware"),
			Reason: record.GetString("reason"), Captured: record.GetDateTime("captured").Time(), Config: object(record, "config"),
		})
	}
	return snapshots, nil
}

func (s *Store) Baseline(device domain.Device) (map[string]any, error) {
	if device.BaselineID == "" {
		return nil, nil
	}
	snapshot, err := s.App.FindRecordById("snapshots", device.BaselineID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if snapshot.GetString("device") != device.ID {
		return nil, nil
	}
	return object(snapshot, "config"), nil
}

func (s *Store) ChannelNotes(id string) (map[string]string, error) {
	records, err := s.App.FindRecordsByFilter("channel_notes", "device = {:id}", "", 0, 0, dbx.Params{"id": id})
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(records))
	for _, record := range records {
		result[record.GetString("component")] = record.GetString("notes")
	}
	return result, nil
}

func (s *Store) SaveChannelNote(id, component, notes string) error {
	return s.App.RunInTransaction(func(app core.App) error {
		record, err := app.FindRecordById("devices", id)
		if err != nil {
			return deviceLookupError(err)
		}
		device := deviceFromRecord(record)
		_, inStatus := device.Status[component]
		_, inConfig := device.Config[component]
		if !inStatus && !inConfig {
			return errors.New("unknown device component")
		}
		note, err := app.FindFirstRecordByFilter("channel_notes", "device = {:id} && component = {:component}", dbx.Params{"id": id, "component": component})
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			collection, err := app.FindCollectionByNameOrId("channel_notes")
			if err != nil {
				return err
			}
			note = core.NewRecord(collection)
			note.Set("device", id)
			note.Set("component", component)
		}
		note.Set("notes", strings.TrimSpace(notes))
		return app.Save(note)
	})
}
