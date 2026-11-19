package inventory

import (
	"database/sql"
	"errors"
	"net/netip"
	"slices"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"shellyficator/internal/domain"
)

var errInvalidNetwork = errors.New("use a private IPv4 range no larger than /24")

func (s *Store) DiscoveryNetworks() ([]string, error) {
	records, err := s.App.FindRecordsByFilter("discovery_networks", "", "", 0, 0)
	if err != nil {
		return nil, err
	}
	prefixes := make([]netip.Prefix, 0, len(records))
	for _, record := range records {
		if prefix, err := netip.ParsePrefix(record.GetString("prefix")); err == nil {
			prefixes = append(prefixes, prefix)
		}
	}
	slices.SortFunc(prefixes, func(a, b netip.Prefix) int { return a.Addr().Compare(b.Addr()) })
	result := make([]string, len(prefixes))
	for i, prefix := range prefixes {
		result[i] = prefix.String()
	}
	return result, nil
}

func (s *Store) RememberNetwork(value string) error {
	prefix := domain.NetworkPrefix(value)
	if prefix == "" {
		return errInvalidNetwork
	}
	return s.App.RunInTransaction(func(app core.App) error {
		existing, err := app.FindRecordsByFilter("discovery_networks", "", "", 0, 0)
		if err != nil {
			return err
		}
		for _, record := range existing {
			if record.GetString("prefix") == prefix {
				return nil
			}
		}
		if len(existing) >= domain.MaxDiscoveryNetworks {
			return domain.ErrNetworkLimit
		}
		collection, err := app.FindCollectionByNameOrId("discovery_networks")
		if err != nil {
			return err
		}
		record := core.NewRecord(collection)
		record.Set("prefix", prefix)
		return app.Save(record)
	})
}

func (s *Store) ForgetNetwork(value string) error {
	prefix := domain.NetworkPrefix(value)
	if prefix == "" {
		return errInvalidNetwork
	}
	record, err := s.App.FindFirstRecordByFilter("discovery_networks", "prefix = {:prefix}", dbx.Params{"prefix": prefix})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.App.Delete(record)
}
