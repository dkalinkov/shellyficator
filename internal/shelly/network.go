package shelly

import (
	"context"
	"shellyficator/internal/domain"
)

// NetworkDiscovery adapts LAN discovery to the application service contract.
type NetworkDiscovery struct{}

func (NetworkDiscovery) Announcements(ctx context.Context) ([]string, error) {
	return Announcements(ctx)
}
func (NetworkDiscovery) Neighbors(ctx context.Context) ([]string, error) { return Neighbors(ctx) }
func (NetworkDiscovery) Targets(target string) ([]string, error)         { return Targets(target) }
func (NetworkDiscovery) Addresses(addresses []string) []string           { return DiscoveryAddresses(addresses) }
func (NetworkDiscovery) LocalNetworks() ([]domain.Network, error)        { return LocalNetworks() }
func (NetworkDiscovery) Explain(err error) string                        { return DiscoveryError(err) }
