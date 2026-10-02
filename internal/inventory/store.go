package inventory

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/shelly"
)

type Store struct{ App core.App }

func New(app core.App) *Store { return &Store{App: app} }

const OfflineGrace = 90 * time.Second
const OfflineFailures = 3

func StatusCurrent(record *core.Record) bool {
	return record.GetBool("reachable") && record.GetBool("status_available") && record.GetInt("consecutive_failures") == 0 && !record.GetBool("identity_mismatch")
}

func ConfigCurrent(record *core.Record) bool {
	return record.GetBool("reachable") && record.GetBool("config_available") && record.GetInt("consecutive_failures") == 0 && !record.GetBool("identity_mismatch")
}

func Object(record *core.Record, name string) map[string]any {
	var result map[string]any
	_ = json.Unmarshal([]byte(record.GetString(name)), &result)
	return result
}

func (s *Store) Devices() ([]*core.Record, error) {
	return s.App.FindRecordsByFilter("devices", "", "name,device_id", 0, 0)
}

func (s *Store) Device(id string) (*core.Record, error) { return s.App.FindRecordById("devices", id) }

func (s *Store) SaveReading(reading shelly.Reading, capture bool) (*core.Record, error) {
	var saved *core.Record
	err := s.App.RunInTransaction(func(app core.App) error {
		record, err := app.FindFirstRecordByFilter("devices", "mac = {:mac}", dbx.Params{"mac": reading.Identity.MAC})
		if err != nil {
			collection, err := app.FindCollectionByNameOrId("devices")
			if err != nil {
				return err
			}
			record = core.NewRecord(collection)
		}
		now := time.Now().UTC()
		for key, value := range map[string]any{"device_id": reading.Identity.ID, "mac": reading.Identity.MAC, "model": reading.Identity.Model, "generation": reading.Identity.Gen, "firmware": reading.Identity.Version, "address": reading.Address, "auth_enabled": reading.Identity.Auth, "reachable": true, "status_available": reading.Status != nil, "problem": reading.Problem, "last_seen": now, "last_checked": now, "consecutive_failures": 0, "identity_mismatch": false} {
			record.Set(key, value)
		}
		if reading.Identity.Name != "" {
			record.Set("name", reading.Identity.Name)
		}
		if reading.Status != nil {
			record.Set("status", reading.Status)
			record.Set("last_status_read", now)
		}
		// Configuration availability follows reads; capture only adds a snapshot.
		if capture || reading.ConfigAttempted || reading.Config != nil {
			record.Set("config_available", reading.Config != nil)
		}
		if reading.Config != nil {
			record.Set("config", reading.Config)
			record.Set("last_captured", time.Now().UTC())
			if sys, ok := reading.Config["sys"].(map[string]any); ok {
				if device, ok := sys["device"].(map[string]any); ok {
					if name, ok := device["name"].(string); ok {
						record.Set("name", name)
					}
				}
			}
		}
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
		}
		saved = record
		return nil
	})
	return saved, err
}

func (s *Store) Failed(id string, problem string, identityMismatch bool) error {
	return s.App.RunInTransaction(func(app core.App) error {
		record, err := app.FindRecordById("devices", id)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		failures := record.GetInt("consecutive_failures") + 1
		record.Set("consecutive_failures", failures)
		// A later timeout cannot hide an identity mismatch; only a validated
		// successful reading clears it.
		identityMismatch = identityMismatch || record.GetBool("identity_mismatch")
		record.Set("identity_mismatch", identityMismatch)
		if identityMismatch {
			record.Set("reachable", false)
			problem = "Address now belongs to a different device. Rediscover this device."
		} else if failures >= OfflineFailures && now.Sub(record.GetDateTime("last_seen").Time()) >= OfflineGrace {
			record.Set("reachable", false)
		}
		record.Set("status_available", false)
		record.Set("config_available", false)
		record.Set("problem", problem)
		record.Set("last_checked", now)
		return app.Save(record)
	})
}

func (s *Store) Metadata(id, room, tags, notes string) error {
	return s.saveMetadata(id, room, tags, &notes)
}

func (s *Store) Snapshots(id string) ([]*core.Record, error) {
	return s.App.FindRecordsByFilter("snapshots", "device = {:id}", "-captured", 50, 0, dbx.Params{"id": id})
}

func (s *Store) MarkBaseline(id string) error {
	return s.App.RunInTransaction(func(app core.App) error {
		record, err := app.FindRecordById("devices", id)
		if err != nil {
			return err
		}
		snapshots, err := app.FindRecordsByFilter("snapshots", "device = {:id}", "-captured", 1, 0, dbx.Params{"id": id})
		if err != nil {
			return err
		}
		if len(snapshots) == 0 || !ConfigCurrent(record) {
			return errors.New("capture configuration before approving a baseline")
		}
		record.Set("baseline", snapshots[0].Id)
		return app.Save(record)
	})
}

func (s *Store) Baseline(record *core.Record) map[string]any {
	if record.GetString("baseline") == "" {
		return nil
	}
	snapshot, err := s.App.FindRecordById("snapshots", record.GetString("baseline"))
	if err != nil || snapshot.GetString("device") != record.Id {
		return nil
	}
	return Object(snapshot, "config")
}

func (s *Store) ChannelNotes(id string) map[string]string {
	result := map[string]string{}
	records, _ := s.App.FindRecordsByFilter("channel_notes", "device = {:id}", "", 0, 0, dbx.Params{"id": id})
	for _, record := range records {
		result[record.GetString("component")] = record.GetString("notes")
	}
	return result
}

func (s *Store) SaveChannelNote(id, component, notes string) error {
	device, err := s.Device(id)
	if err != nil {
		return err
	}
	_, inStatus := Object(device, "status")[component]
	_, inConfig := Object(device, "config")[component]
	if !inStatus && !inConfig {
		return errors.New("unknown device component")
	}
	record, err := s.App.FindFirstRecordByFilter("channel_notes", "device = {:id} && component = {:component}", dbx.Params{"id": id, "component": component})
	if err != nil {
		collection, err := s.App.FindCollectionByNameOrId("channel_notes")
		if err != nil {
			return err
		}
		record = core.NewRecord(collection)
		record.Set("device", id)
		record.Set("component", component)
	}
	record.Set("notes", strings.TrimSpace(notes))
	return s.App.Save(record)
}

type Difference struct{ Path, Before, After string }

func Differences(baseline, current map[string]any) []Difference {
	before, after := map[string]string{}, map[string]string{}
	flatten("", baseline, before)
	flatten("", current, after)
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	var differences []Difference
	for key := range keys {
		old, hadOld := before[key]
		now, hasNow := after[key]
		if hadOld && hasNow && old == now {
			continue
		}
		if !hadOld {
			old = "Not present"
		}
		if !hasNow {
			now = "Not present"
		}
		differences = append(differences, Difference{Path: key, Before: old, After: now})
	}
	sort.Slice(differences, func(i, j int) bool { return differences[i].Path < differences[j].Path })
	return differences
}

func flatten(prefix string, object map[string]any, result map[string]string) {
	for key, value := range object {
		path := key
		if prefix != "" {
			path = prefix + " / " + key
		}
		if nested, ok := value.(map[string]any); ok && len(nested) > 0 {
			flatten(path, nested, result)
			continue
		}
		bytes, _ := json.Marshal(value)
		result[path] = string(bytes)
	}
}

func FormatTime(value string) string {
	parsed, err := time.Parse("2006-01-02 15:04:05.000Z", value)
	if err != nil {
		return "Never"
	}
	return parsed.Local().Format("02 Jan, 15:04:05")
}

func Display(value any) string {
	if value == nil {
		return "Not reported"
	}
	if number, ok := value.(float64); ok {
		return fmt.Sprintf("%.1f", number)
	}
	return fmt.Sprint(value)
}
