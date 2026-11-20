package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"shellyficator/internal/domain"
)

type actionDevices struct {
	Devices
	ctx    context.Context
	id     string
	err    error
	result domain.RefreshResult
}

func (s *actionDevices) ApproveBaseline(ctx context.Context, id string) (domain.Device, error) {
	s.ctx, s.id = ctx, id
	return domain.Device{}, s.err
}

func (s *actionDevices) RefreshDevice(ctx context.Context, id string) (domain.RefreshResult, error) {
	s.ctx, s.id = ctx, id
	return s.result, s.err
}

func TestRefreshHandlerPreservesReadAndStorageFailureResponses(t *testing.T) {
	deviceLookupFailure := errors.New("device lookup failed")
	failedCheckSaveFailure := errors.New("failed check persistence failed")
	readingSaveFailure := errors.New("reading persistence failed")
	for _, test := range []struct {
		name, notice string
		err          error
		available    bool
	}{
		{"success", "Status+refreshed.", nil, true},
		{"unavailable status", "Device+reached", nil, false},
		{"device read failure", "Refresh+failed", errors.Join(domain.ErrDeviceRead, domain.ErrIdentity), false},
		{"missing device", "", domain.ErrNotFound, false},
		{"device lookup failure", "", deviceLookupFailure, false},
		{"failed check save failure", "", failedCheckSaveFailure, false},
		{"reading save failure", "", readingSaveFailure, false},
		{"caller cancellation", "", context.Canceled, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &actionDevices{err: test.err, result: domain.RefreshResult{StatusAvailable: test.available}}
			server := &Server{devices: service}
			request := httptest.NewRequest(http.MethodPost, "/devices/device/refresh", nil)
			request.SetPathValue("id", "device")
			if errors.Is(test.err, context.Canceled) {
				ctx, cancel := context.WithCancel(request.Context())
				cancel()
				request = request.WithContext(ctx)
			}
			response := httptest.NewRecorder()
			event := &core.RequestEvent{Event: router.Event{Request: request, Response: response}}
			err := server.refreshDevice(event)
			if errors.Is(test.err, domain.ErrNotFound) {
				var apiError *router.ApiError
				if !errors.As(err, &apiError) || apiError.Status != http.StatusNotFound {
					t.Fatalf("missing device lost its 404 response: %v", err)
				}
			} else if test.notice != "" {
				if err != nil || response.Code != http.StatusSeeOther || !strings.Contains(response.Header().Get("Location"), test.notice) {
					t.Fatalf("unexpected refresh response: err=%v status=%d location=%s", err, response.Code, response.Header().Get("Location"))
				}
			} else if !errors.Is(err, test.err) {
				t.Fatalf("storage or cancellation failure was hidden: got %v, want %v", err, test.err)
			}
			if service.id != "device" || service.ctx != request.Context() {
				t.Fatal("handler changed the caller context or device identity")
			}
		})
	}
}

func TestBaselineHandlerPreservesReadAndSaveFailureResponses(t *testing.T) {
	for _, test := range []struct {
		name, notice string
		err          error
		status       int
	}{
		{"success", "Baseline+updated.", nil, http.StatusSeeOther},
		{"read failure", "Baseline+unchanged", domain.ErrConfigUnavailable, http.StatusSeeOther},
		{"save failure", "", errors.Join(domain.ErrBaselineSave, errors.New("database write failed")), http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &actionDevices{err: test.err}
			server := &Server{devices: service}
			request := httptest.NewRequest(http.MethodPost, "/devices/device/baseline", nil)
			request.SetPathValue("id", "device")
			response := httptest.NewRecorder()
			event := &core.RequestEvent{Event: router.Event{Request: request, Response: response}}
			err := server.baseline(event)
			if test.status == http.StatusBadRequest {
				var apiError *router.ApiError
				if !errors.As(err, &apiError) || apiError.Status != test.status || strings.TrimSuffix(apiError.Message, ".") != "Could not approve baseline" {
					t.Fatalf("save error was disguised as device read failure: %v", err)
				}
			} else if err != nil || response.Code != test.status || !strings.Contains(response.Header().Get("Location"), test.notice) {
				t.Fatalf("unexpected baseline response: err=%v status=%d location=%s", err, response.Code, response.Header().Get("Location"))
			}
			if service.id != "device" || service.ctx != request.Context() {
				t.Fatal("handler changed the caller context or device identity")
			}
		})
	}
}

type failingAccounts struct {
	Accounts
	err error
}

func (s failingAccounts) Authenticate(string, string) (string, error) {
	return "", s.err
}

func TestLoginHandlerSeparatesInvalidCredentialsFromServiceFailure(t *testing.T) {
	templates, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		err  error
	}{
		{"invalid credentials", domain.ErrInvalidCredentials},
		{"account storage failure", errors.New("database unavailable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := &Server{accounts: failingAccounts{err: test.err}, templates: templates, loginAttempts: map[string]loginAttempt{}}
			response := httptest.NewRecorder()
			event := &core.RequestEvent{Event: router.Event{Request: httptest.NewRequest(http.MethodPost, "/login", nil), Response: response}}
			err := server.login(event)
			if errors.Is(test.err, domain.ErrInvalidCredentials) {
				if err != nil || response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Email or password is incorrect.") {
					t.Fatalf("invalid credentials lost their form response: %v %s", err, response.Body.String())
				}
			} else {
				var apiError *router.ApiError
				if !errors.As(err, &apiError) || apiError.Status != http.StatusInternalServerError {
					t.Fatalf("service failure was disguised as invalid credentials: %v", err)
				}
			}
			for _, cookie := range response.Result().Cookies() {
				if cookie.Name == "shelly_session" {
					t.Fatal("failed authentication issued a session")
				}
			}
		})
	}
}
