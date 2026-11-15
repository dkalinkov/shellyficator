package domain

import "time"

type FirmwareSelection struct {
	DeviceID, Stage, FromVersion, TargetVersion string
}

type FirmwareUpdate struct {
	ID, Stage, FromVersion, TargetVersion string
	Phase, Message                        string
	UpdatedAt                             time.Time
}

func (u *FirmwareUpdate) Active() bool {
	if u == nil {
		return false
	}
	switch u.Phase {
	case "queued", "capturing", "starting", "verifying":
		return true
	}
	return false
}
