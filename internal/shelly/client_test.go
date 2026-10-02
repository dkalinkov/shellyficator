package shelly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
			_, _ = w.Write([]byte(`{"wifi":{"sta":{"pass":"sensitive"}},"switch:0":{"in_mode":"momentary"},"extra":[[{"password":"sensitive"}],"mqtt://admin:sensitive@192.168.1.2"]}`))
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
	if reading.Config["extra"].([]any)[1] != "mqtt://192.168.1.2" {
		t.Fatal("URL redaction discarded the public connection address")
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

func TestRefreshRetriesTransientReadsOnce(t *testing.T) {
	for _, path := range []string{"/shelly", "/rpc/Shelly.GetStatus", "/rpc/Shelly.GetConfig"} {
		t.Run(path, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == path && attempts.Add(1) == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				switch r.URL.Path {
				case "/shelly":
					_, _ = w.Write([]byte(`{"id":"test","mac":"AABBCCDDEEFF","gen":4}`))
				case "/rpc/Shelly.GetStatus":
					_, _ = w.Write([]byte(`{"switch:0":{"output":false}}`))
				case "/rpc/Shelly.GetConfig":
					_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"momentary"}}`))
				}
			}))
			defer server.Close()
			reading, err := NewClient("").Read(context.Background(), server.URL, "AABBCCDDEEFF", true)
			if err != nil || attempts.Load() != 2 || reading.Problem != "" || reading.Status == nil || reading.Config == nil {
				t.Fatalf("transient read did not recover: reading=%+v attempts=%d err=%v", reading, attempts.Load(), err)
			}
		})
	}
}

func TestFailedStatusDoesNotReturnPartialDataOrRetryAccessErrors(t *testing.T) {
	for _, mode := range []string{"authentication", "decode"} {
		t.Run(mode, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/shelly" {
					_, _ = w.Write([]byte(`{"id":"test","mac":"AABBCCDDEEFF","gen":4}`))
					return
				}
				attempts.Add(1)
				if mode == "authentication" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				_, _ = w.Write([]byte(`{"switch:0":{"output":true},"invalid":1e999}`))
			}))
			defer server.Close()
			reading, err := NewClient("").Read(context.Background(), server.URL, "AABBCCDDEEFF", false)
			if err != nil || reading.Status != nil || reading.Problem == "" || attempts.Load() != 1 || reading.Identity.MAC != "AABBCCDDEEFF" {
				t.Fatalf("status error lost reachable identity, returned partial data, or retried: %+v err=%v", reading, err)
			}
			if mode == "authentication" && reading.Problem != ErrAuthentication.Error() {
				t.Fatal("authentication failure was described as connectivity failure")
			}
		})
	}
}

func TestReadPreservesAuthenticationFailureAcrossBothReads(t *testing.T) {
	for _, statuses := range [][2]int{{http.StatusForbidden, http.StatusNotFound}, {http.StatusNotFound, http.StatusForbidden}} {
		t.Run(fmt.Sprintf("status_%d_config_%d", statuses[0], statuses[1]), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/shelly":
					_, _ = w.Write([]byte(`{"id":"test","mac":"AABBCCDDEEFF","gen":4}`))
				case "/rpc/Shelly.GetStatus":
					w.WriteHeader(statuses[0])
				case "/rpc/Shelly.GetConfig":
					w.WriteHeader(statuses[1])
				}
			}))
			defer server.Close()
			reading, err := NewClient("").Read(context.Background(), server.URL, "AABBCCDDEEFF", true)
			if err != nil || reading.Problem != ErrAuthentication.Error() || reading.Status != nil || reading.Config != nil || !reading.ConfigAttempted {
				t.Fatalf("authentication failure hidden by another read failure: %+v err=%v", reading, err)
			}
		})
	}
}

func TestCanceledReadDoesNotRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		cancel()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	_, err := NewClient("").Read(ctx, server.URL, "AABBCCDDEEFF", false)
	if !errors.Is(err, context.Canceled) || requests.Load() != 1 {
		t.Fatalf("canceled check retried or lost cancellation: err=%v requests=%d", err, requests.Load())
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
