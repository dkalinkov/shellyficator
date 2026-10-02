package shelly

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadIdentityBeforeAuthenticatedRequests(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/shelly" {
			_, _ = w.Write([]byte(`{"id":"shelly-test","mac":"AA:BB:CC:DD:EE:FF","gen":4}`))
			return
		}
		requests++
	}))
	defer server.Close()
	_, err := NewClient("unused").Read(context.Background(), server.URL, "112233445566", true)
	if !errors.Is(err, ErrIdentity) || requests != 0 {
		t.Fatalf("identity mismatch should stop management reads; err=%v requests=%d", err, requests)
	}
}

func TestDiscoveryRetriesInterruptedIdentityButDoesNotRetryInvalidDevices(t *testing.T) {
	for _, mode := range []string{"interrupted", "invalid", "unauthorized"} {
		t.Run(mode, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count := requests.Add(1)
				if r.URL.Path != "/shelly" {
					_, _ = w.Write([]byte(`{}`))
					return
				}
				if mode == "interrupted" && count == 1 {
					w.Header().Set("Content-Length", "512")
					_, _ = w.Write([]byte(`{"id":`))
					return
				}
				if mode == "unauthorized" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if mode == "invalid" {
					_, _ = w.Write([]byte(`{"model":"printer"}`))
					return
				}
				_, _ = w.Write([]byte(`{"id":"retry-device","mac":"AABBCCDDEEFF","gen":4}`))
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			reading, err := NewClient("").Discover(ctx, server.URL)
			if mode == "interrupted" {
				if err != nil || reading.Identity.ID != "retry-device" || requests.Load() != 4 {
					t.Fatalf("interrupted discovery not recovered: err=%v requests=%d", err, requests.Load())
				}
			} else if err == nil || requests.Load() != 1 {
				t.Fatalf("invalid device retried: err=%v requests=%d", err, requests.Load())
			}
		})
	}
}

func TestCaptureRedactsSecretsAndKeepsStatelessInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"test","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{"input:0":{"state":null},"switch:0":{"apower":0}}`))
		case "/rpc/Shelly.GetConfig":
			_, _ = w.Write([]byte(`{"wifi":{"sta":{"pass":"sensitive"}},"switch:0":{"in_mode":"momentary"}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	reading, err := NewClient("").Read(context.Background(), server.URL, "", true)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(reading.Config)
	if strings.Contains(string(encoded), "sensitive") {
		t.Fatal("secret present in capture")
	}
	if reading.Status["input:0"].(map[string]any)["state"] != nil {
		t.Fatal("stateless input changed")
	}
	if !reading.ConfigAttempted {
		t.Fatal("configuration read not recorded")
	}
}

func TestReadRejectsPartialConfigurationOnDecodeFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			_, _ = w.Write([]byte(`{"id":"test","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{"switch:0":{"output":true}}`))
		case "/rpc/Shelly.GetConfig":
			// Valid syntax, but the overflowing number prevents a complete decode.
			_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"detached"},"invalid":1e999}`))
		}
	}))
	defer server.Close()
	reading, err := NewClient("").Read(context.Background(), server.URL, "", true)
	if err != nil || !reading.ConfigAttempted || reading.Config != nil || reading.Problem == "" || reading.Status["switch:0"].(map[string]any)["output"] != true {
		t.Fatalf("partial configuration treated as available or valid status lost: %+v err=%v", reading, err)
	}
}

func TestTargetsBoundsAndAddresses(t *testing.T) {
	for _, value := range []string{"10.0.0.0/16", "8.8.8.0/24", "file:///etc/passwd", "http://admin:password@192.168.1.2", "192.168.1.2/rpc/Switch.Set"} {
		if _, err := Targets(value); err == nil {
			t.Errorf("accepted invalid target %q", value)
		}
	}
	addresses, err := Targets("192.168.1.5/24")
	if err != nil || len(addresses) != 254 || addresses[0] != "http://192.168.1.1" || addresses[253] != "http://192.168.1.254" {
		t.Fatalf("unexpected range: %v, %d", err, len(addresses))
	}
}
