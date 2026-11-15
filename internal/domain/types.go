// Package domain contains shared application concepts and pure business rules.
// It depends only on the standard library.
package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound           = errors.New("device not found")
	ErrIdentity           = errors.New("the device at this address has a different identity")
	ErrDeviceRead         = errors.New("device read failed")
	ErrAuthentication     = errors.New("authentication required or shared device password rejected")
	ErrConfigUnavailable  = errors.New("configuration could not be read; check connectivity and shared device credentials")
	ErrBaselineSave       = errors.New("could not approve baseline")
	ErrClosed             = errors.New("device service is closed")
	ErrInvalidCredentials = errors.New("email or password is incorrect")
	ErrSetupCompleted     = errors.New("setup already completed")
)

type Identity struct {
	ID, MAC, Name, Model, Version string
	Gen                           int
	Auth                          bool
}

type Reading struct {
	Identity        Identity
	Address         string
	Status, Config  map[string]any
	ConfigAttempted bool
	Problem         string
	FirmwareCheck   *FirmwareCheck
	StatusReadAt    time.Time
}

type Device struct {
	ID, DeviceID, MAC, Name, Model, Firmware, Address   string
	Generation                                          int
	AuthEnabled                                         bool
	Room                                                string
	Tags                                                []string
	Notes                                               string
	Reachable, StatusAvailable, ConfigAvailable         bool
	ConsecutiveFailures                                 int
	IdentityMismatch                                    bool
	Problem                                             string
	LastSeen, LastChecked, LastCaptured, LastStatusRead time.Time
	Status, Config                                      map[string]any
	BaselineID                                          string
	FirmwareCheck                                       *FirmwareCheck
}

// FirmwareCheck is the result of an explicit device-side update check. Polling
// does not erase its error, so a failed check remains visible until retried.
type FirmwareCheck struct {
	CheckedAt time.Time      `json:"checked_at"`
	Firmware  string         `json:"firmware"`
	Available map[string]any `json:"available"`
	Problem   string         `json:"problem,omitempty"`
}

type Snapshot struct {
	ID, DeviceID, Firmware, Reason string
	Captured                       time.Time
	Config                         map[string]any
}

type InventoryEntry struct {
	Device       Device
	Baseline     map[string]any
	ChannelNotes map[string]string
}

type DeviceDetails struct {
	InventoryEntry
	Snapshots           []Snapshot
	OrganizationOptions OrganizationOptions
}

type OrganizationOptions struct{ Rooms, Tags []string }

type Discovery struct {
	Running                 bool
	Total, Completed, Found int
	Target, Message, Stage  string
}

type Network struct{ Interface, Prefix string }

type RefreshResult struct{ StatusAvailable bool }

// SettingValue records presence separately from null, false, and zero.
type SettingValue struct {
	Present bool
	Value   any
}

type Difference struct {
	Path          string
	Before, After SettingValue
}

type Operator struct{ ID, Email string }

// ExternalIdentity is supplied only after validating the provider's OIDC response.
type ExternalIdentity struct {
	Issuer, Subject, Email string
	EmailVerified          bool
}

type Session struct {
	Operator Operator
	Token    string
}
