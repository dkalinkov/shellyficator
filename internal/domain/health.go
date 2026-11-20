package domain

import "time"

const OfflineGrace = 90 * time.Second
const OfflineFailures = 3

func (d Device) StatusCurrent() bool {
	return d.Reachable && d.StatusAvailable && d.ConsecutiveFailures == 0 && !d.IdentityMismatch
}

func (d Device) ConfigCurrent() bool {
	return d.Reachable && d.ConfigAvailable && d.ConsecutiveFailures == 0 && !d.IdentityMismatch
}

func (d Device) FailedCheck(problem string, identityMismatch bool, now time.Time) Device {
	d.ConsecutiveFailures++
	d.IdentityMismatch = d.IdentityMismatch || identityMismatch
	if d.IdentityMismatch {
		d.Reachable = false
		problem = "Address now belongs to a different device. Rediscover this device."
	} else if d.ConsecutiveFailures >= OfflineFailures && now.Sub(d.LastSeen) >= OfflineGrace {
		d.Reachable = false
	}
	d.StatusAvailable, d.ConfigAvailable = false, false
	d.Problem, d.LastChecked = problem, now
	return d
}

// ApplyReading preserves local metadata and cached values absent from this read.
func (d Device) ApplyReading(r Reading, capture bool, now time.Time) Device {
	if d.Firmware != r.Identity.Version {
		d.FirmwareCheck = nil
	}
	if r.FirmwareCheck != nil {
		d.FirmwareCheck = r.FirmwareCheck
	}
	d.DeviceID, d.MAC, d.Model = r.Identity.ID, r.Identity.MAC, r.Identity.Model
	d.Generation, d.Firmware, d.Address = r.Identity.Gen, r.Identity.Version, r.Address
	d.AuthEnabled, d.Reachable = r.Identity.Auth, true
	d.StatusAvailable = r.Status != nil
	d.Problem, d.LastSeen, d.LastChecked = r.Problem, now, now
	d.ConsecutiveFailures, d.IdentityMismatch = 0, false
	if r.Identity.Name != "" {
		d.Name = r.Identity.Name
	}
	if r.Status != nil {
		d.Status, d.LastStatusRead = r.Status, now
		if !r.StatusReadAt.IsZero() {
			d.LastStatusRead = r.StatusReadAt
		}
	}
	if capture || r.ConfigAttempted || r.Config != nil {
		d.ConfigAvailable = r.Config != nil
	}
	if r.Config != nil {
		d.Config, d.LastCaptured = r.Config, now
		if sys, ok := r.Config["sys"].(map[string]any); ok {
			if device, ok := sys["device"].(map[string]any); ok {
				if name, ok := device["name"].(string); ok {
					d.Name = name
				}
			}
		}
	}
	return d
}
