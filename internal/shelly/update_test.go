package shelly

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"shellyficator/internal/domain"
)

// A separate loopback address isolates test devices from workstation HTTP
// readiness probes against newly opened 127.0.0.1 ports.
func updateTestDevice(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func TestFirmwareCommandIsIdentityCheckedAndNeverRetried(t *testing.T) {
	for _, mode := range []string{"accepted", "rejected", "HTTP error", "lost response", "wrong identity", "invalid stage"} {
		t.Run(mode, func(t *testing.T) {
			var writes atomic.Int32
			server := updateTestDevice(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/shelly" {
					mac := "AABBCCDDEEFF"
					if mode == "wrong identity" {
						mac = "000000000001"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"id": "test-device", "mac": mac, "gen": 4, "ver": "2.1.0"})
					return
				}
				if r.Method != http.MethodPost || r.URL.Path != "/rpc" {
					t.Errorf("unexpected device request %s %s", r.Method, r.URL.Path)
					return
				}
				writes.Add(1)
				var body struct {
					Method string            `json:"method"`
					Params map[string]string `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Method != "Shelly.Update" || body.Params["stage"] != "beta" || len(body.Params) != 1 {
					t.Errorf("wrong update command: %+v %v", body, err)
				}
				switch mode {
				case "rejected":
					_, _ = w.Write([]byte(`{"id":1,"error":{"code":-114,"message":"Update unavailable"}}`))
				case "HTTP error":
					w.WriteHeader(503)
				case "lost response":
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						_ = conn.Close()
					}
				default:
					_, _ = w.Write([]byte(`{"id":1,"result":null}`))
				}
			})
			stage := "beta"
			if mode == "invalid stage" {
				stage = "custom"
			}
			err := NewClient("").StartFirmwareUpdate(context.Background(), server.URL, "AABBCCDDEEFF", stage)
			if mode == "accepted" && err != nil {
				t.Fatal(err)
			}
			if mode != "accepted" && err == nil {
				t.Fatal("failed command reported acceptance")
			}
			if mode == "rejected" || mode == "HTTP error" || mode == "wrong identity" {
				if !errors.Is(err, domain.ErrFirmwareRejected) {
					t.Fatalf("explicit rejection became an ambiguous request: %v", err)
				}
			}
			want := int32(1)
			if mode == "wrong identity" || mode == "invalid stage" {
				want = 0
			}
			if writes.Load() != want {
				t.Fatalf("update was retried or sent to another device: %d", writes.Load())
			}
		})
	}
}
