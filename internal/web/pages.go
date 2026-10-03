package web

import (
	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/devices"
	"shellyficator/internal/inventory"
	"shellyficator/internal/shelly"
)

type PageMeta struct {
	Title, CSRF, Notice, Version string
}

type AuthPage struct {
	PageMeta
	Error string
}

type InventoryPage struct {
	PageMeta
	Query, Room, GroupBy                   string
	Devices                                []Device
	RoomGroups                             []RoomGroup
	Rooms, Tags, SelectedTags              []string
	Total, Reachable, Changed, Unprotected int
	Discovery                              devices.Discovery
	Networks                               []shelly.Network
}

type DevicePage struct {
	PageMeta
	Device      *core.Record
	Name        string
	Rooms, Tags []string
	Detail      DeviceDetail
	Differences []inventory.Difference
	HasBaseline bool
	Snapshots   []*core.Record
}

type DiscoveryPage struct {
	Discovery devices.Discovery
}

type Device struct {
	Record *core.Record
	Name   string
	Table  DeviceTable
}

type RoomGroup struct {
	Room    string
	Devices []Device
}

type Metric struct{ Label, Value string }
