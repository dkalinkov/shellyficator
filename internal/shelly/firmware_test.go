package shelly

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shellyficator/internal/domain"
)

func TestFirmwareCheckDistinguishesAvailabilityFromErrors(t *testing.T) {
	for _, test := range []struct {
		name, body, problem string
		status              int
	}{
		{"available", `{"stable":{"version":"2.2.0"},"beta":{"version":"2.3.0-beta1"}}`, "", 200},
		{"no update", `{}`, "", 200},
		{"RPC error", `{"code":-114,"message":"DNS lookup failed"}`, "DNS lookup failed", 200},
		{"RPC envelope error", `{"error":{"code":-114,"message":"Download unavailable"}}`, "Download unavailable", 200},
		{"HTTP RPC error", `{"code":-114,"message":"Connection refused"}`, "Connection refused", 500},
		{"authentication", `{}`, "shared device password", 403},
		{"unsupported", `{}`, "does not support", 404},
		{"invalid", `not JSON`, "valid update check", 200},
		{"null", `null`, "valid update check", 200},
		{"invalid offer", `{"stable":{"version":null}}`, "valid update check", 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			var checks, configReads, writes int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					writes++
				}
				switch r.URL.Path {
				case "/shelly":
					_, _ = w.Write([]byte(`{"id":"device","mac":"AABBCCDDEEFF","gen":4,"ver":"2.1.0"}`))
				case "/rpc/Shelly.GetStatus":
					_, _ = w.Write([]byte(`{"sys":{"available_updates":{}}}`))
				case "/rpc/Shelly.CheckForUpdate":
					checks++
					w.WriteHeader(test.status)
					_, _ = w.Write([]byte(test.body))
				case "/rpc/Shelly.GetConfig":
					configReads++
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
				}
			}))
			defer server.Close()
			reading, err := NewClient("").CheckForUpdate(context.Background(), server.URL, "AABBCCDDEEFF")
			if err != nil || reading.FirmwareCheck == nil || checks != 1 || configReads != 0 || writes != 0 {
				t.Fatalf("check failed or contacted unexpected endpoints: %+v err=%v checks=%d config=%d writes=%d", reading, err, checks, configReads, writes)
			}
			check := reading.FirmwareCheck
			if check.Firmware != "2.1.0" || check.CheckedAt.IsZero() || reading.Status == nil {
				t.Fatalf("check lost device identity/status/time: %+v", check)
			}
			if reading.StatusReadAt.IsZero() || check.CheckedAt.Before(reading.StatusReadAt) {
				t.Fatalf("availability check lost the ordering of the earlier status read: %+v", reading)
			}
			if test.problem == "" {
				if check.Problem != "" || check.Available == nil {
					t.Fatalf("successful availability check reported failure: %+v", check)
				}
			} else if !strings.Contains(check.Problem, test.problem) || check.Available != nil {
				t.Fatalf("failed check reported success or lost the reason: %+v", check)
			}
		})
	}
}

func TestFirmwareCheckVerifiesIdentityBeforeContactingUpdateServer(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"id":"different","mac":"000000000001","gen":4}`))
	}))
	defer server.Close()
	_, err := NewClient("").CheckForUpdate(context.Background(), server.URL, "AABBCCDDEEFF")
	if !errors.Is(err, domain.ErrIdentity) || requests != 1 {
		t.Fatalf("identity mismatch was ignored: %v requests=%d", err, requests)
	}
}

func TestFirmwareCheckAllowsTimeForTheUpstreamResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{}`))
		case "/rpc/Shelly.CheckForUpdate":
			timer := time.NewTimer(5200 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-r.Context().Done():
				return
			case <-timer.C:
				_, _ = w.Write([]byte(`{}`))
			}
		}
	}))
	defer server.Close()
	reading, err := NewClient("").CheckForUpdate(context.Background(), server.URL, "AABBCCDDEEFF")
	if err != nil || reading.FirmwareCheck == nil || reading.FirmwareCheck.Problem != "" || reading.FirmwareCheck.Available == nil {
		t.Fatalf("upstream check used the short local-status deadline: %+v err=%v", reading.FirmwareCheck, err)
	}
}
