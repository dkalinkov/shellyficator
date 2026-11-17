package inventory

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
)

func (s *Store) WatchPreferences() (domain.WatchPreferences, error) {
	records, err := s.App.FindRecordsByFilter("watch_preferences", "", "", 0, 0)
	if err != nil {
		return nil, err
	}
	result := domain.WatchPreferences{}
	for _, record := range records {
		var policy domain.WatchPolicy
		if err := json.Unmarshal([]byte(record.GetString("policy")), &policy); err != nil {
			return nil, err
		}
		scope := record.GetString("scope")
		if scope == "global" {
			scope = ""
		}
		result[scope] = policy
	}
	return result, nil
}

// A nil policy removes an override; the global default is always saved explicitly.
func (s *Store) SaveWatchPolicy(deviceID string, policy domain.WatchPolicy) error {
	for group := range policy {
		known := false
		for _, valid := range domain.WatchGroups {
			known = known || group == valid
		}
		if !known {
			return errors.New("unknown watch group")
		}
	}
	return s.App.RunInTransaction(func(app core.App) error {
		scope := deviceID
		if scope == "" {
			scope = "global"
		} else {
			if _, err := app.FindRecordById("devices", deviceID); err != nil {
				return deviceLookupError(err)
			}
		}
		record, err := app.FindFirstRecordByFilter("watch_preferences", "scope = {:scope}", dbx.Params{"scope": scope})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if policy == nil {
			if record != nil {
				return app.Delete(record)
			}
			return nil
		}
		if record == nil {
			collection, err := app.FindCollectionByNameOrId("watch_preferences")
			if err != nil {
				return err
			}
			record = core.NewRecord(collection)
			record.Set("scope", scope)
		}
		// Save all groups explicitly, including an all-record-only selection.
		normalized := domain.WatchPolicy{}
		for _, group := range domain.WatchGroups {
			normalized[group] = policy.Warn(group)
		}
		record.Set("policy", normalized)
		return app.Save(record)
	})
}
