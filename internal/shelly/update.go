package shelly

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"shellyficator/internal/domain"
)

// StartFirmwareUpdate sends exactly one authenticated update command. A lost
// response is ambiguous, so callers verify the installed version instead of
// retrying the write. Only the built-in stable and beta channels are supported.
func (c *Client) StartFirmwareUpdate(ctx context.Context, address, expectedMAC, stage string) error {
	if stage != "stable" && stage != "beta" {
		return domain.ErrFirmwareSelection
	}
	reading, err := c.identify(ctx, address, expectedMAC)
	if err != nil {
		return errors.Join(domain.ErrFirmwareRejected, err)
	}
	body, _ := json.Marshal(map[string]any{"id": 1, "method": "Shelly.Update", "params": map[string]string{"stage": stage}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reading.Address+"/rpc", bytes.NewReader(body))
	if err != nil {
		return errors.Join(domain.ErrFirmwareRejected, err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.firmware.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return errors.Join(domain.ErrFirmwareRejected, domain.ErrAuthentication)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1048577))
	if err != nil {
		return err
	}
	var result struct {
		Error   *rpcError       `json:"error"`
		Code    *int            `json:"code"`
		Message string          `json:"message"`
		Result  json.RawMessage `json:"result"`
	}
	decodeErr := json.Unmarshal(data, &result)
	if decodeErr == nil && result.Error != nil {
		return fmt.Errorf("%w: %s", domain.ErrFirmwareRejected, result.Error)
	}
	if decodeErr == nil && result.Code != nil && result.Message != "" {
		return fmt.Errorf("%w: %s", domain.ErrFirmwareRejected, result.Message)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: HTTP %d", domain.ErrFirmwareRejected, response.StatusCode)
	}
	if decodeErr != nil || len(data) > 1048576 || len(result.Result) == 0 {
		return errors.New("update response was not confirmed; verify the installed version")
	}
	return nil
}
