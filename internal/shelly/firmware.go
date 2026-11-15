package shelly

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"shellyficator/internal/domain"
)

// CheckForUpdate verifies identity before asking the device to contact its
// update server. It never installs firmware or changes device configuration.
func (c *Client) CheckForUpdate(ctx context.Context, address, expectedMAC string) (domain.Reading, error) {
	reading, err := c.Read(ctx, address, expectedMAC, false)
	if err != nil {
		return reading, err
	}
	var available map[string]any
	err = c.getOnce(ctx, c.firmware, reading.Address+"/rpc/Shelly.CheckForUpdate", &available)
	check := &domain.FirmwareCheck{CheckedAt: time.Now().UTC(), Firmware: reading.Identity.Version}
	if err == nil && available == nil {
		err = errors.New("device returned an empty update response")
	}
	if err == nil {
		for _, stage := range []string{"stable", "beta"} {
			if offer, present := available[stage]; present {
				release, _ := offer.(map[string]any)
				version, _ := release["version"].(string)
				if strings.TrimSpace(version) == "" {
					err = errors.New("device returned an invalid firmware offer")
				}
			}
		}
	}
	if err != nil {
		check.Problem = firmwareCheckError(err)
	} else {
		check.Available = available
	}
	reading.FirmwareCheck = check
	return reading, nil
}

func firmwareCheckError(err error) string {
	if errors.Is(err, domain.ErrAuthentication) {
		return "Update check was rejected. Check the shared device password."
	}
	var rpc *rpcError
	if errors.As(err, &rpc) {
		return fmt.Sprintf("Device update check failed (%d): %s", rpc.Code, rpc.Message)
	}
	var response *httpError
	if errors.As(err, &response) {
		if response.code == 404 {
			return "This firmware does not support checking for updates through the device API. Open the device UI."
		}
		return fmt.Sprintf("Device update check failed (HTTP %d). Check the device UI and its internet connection.", response.code)
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() || errors.Is(err, context.DeadlineExceeded) {
		return "Update check timed out. Check the device's internet connection, DNS and clock, then retry."
	}
	return "Could not read a valid update check. Check the device UI and retry."
}
