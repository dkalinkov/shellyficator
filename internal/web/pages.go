package web

import (
	"shellyficator/internal/domain"
)

type PageMeta struct {
	Title, CSRF, Notice, NoticeLevel, Version string
}

type AuthPage struct {
	PageMeta
	Error       string
	OIDCEnabled bool
}

type InventoryPage struct {
	PageMeta
	Query, Room, GroupBy                   string
	Devices                                []Device
	RoomGroups                             []RoomGroup
	Rooms, Tags, SelectedTags              []string
	Total, Reachable, Changed, Unprotected int
	Updates                                int
	FirmwareRunning                        bool
	Discovery                              domain.Discovery
	Networks                               []domain.Network
}

type DevicePage struct {
	PageMeta
	Device                   DeviceView
	Name                     string
	Rooms, Tags              []string
	Detail                   DeviceDetail
	Differences              []DifferenceView
	InformationalDifferences []DifferenceView
	HasBaseline              bool
	FirmwareRunning          bool
	Snapshots                []SnapshotView
}

type DiscoveryPage struct {
	Discovery domain.Discovery
}

type Device struct {
	Info  DeviceView
	Name  string
	Table DeviceTable
}

type DeviceView struct {
	ID, Name, DeviceID, Model, MAC, Firmware, Address string
	Room, TagsText, Notes, Problem, BaselineID        string
	Tags                                              []string
	Generation                                        int
	AuthEnabled                                       bool
	LastSeen, LastChecked, LastCaptured               string
	HasCapture                                        bool
}

type SnapshotView struct {
	ID, Reason, Firmware, Captured, CapturedISO string
}

type DifferenceView struct {
	Path, Before, After string
}

type RoomGroup struct {
	Index   int
	Room    string
	Devices []Device
}

type Metric struct{ Label, Value string }

func (m Metric) Warning() bool {
	return m.Label == "Locked" && m.Value == "Yes" || m.Label == "Enabled" && m.Value == "No"
}
