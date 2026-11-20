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

type refreshFailure struct {
	Devices
	err error
}

func (s refreshFailure) RefreshDevice(context.Context, string) (bool, error) {
	return false, s.err
}

func TestRefreshHandlerSeparatesMissingDevicesFromStorageFailure(t *testing.T) {
	storageFailure := errors.New("device lookup failed")
	for _, failure := range []error{domain.ErrNotFound, storageFailure} {
		server := &Server{devices: refreshFailure{err: failure}}
		request := httptest.NewRequest(http.MethodPost, "/devices/device/refresh", nil)
		request.SetPathValue("id", "device")
		response := httptest.NewRecorder()
		event := &core.RequestEvent{Event: router.Event{Request: request, Response: response}}
		err := server.refreshDevice(event)
		if errors.Is(failure, domain.ErrNotFound) {
			var apiError *router.ApiError
			if !errors.As(err, &apiError) || apiError.Status != http.StatusNotFound {
				t.Fatalf("missing device lost its 404 response: %v", err)
			}
		} else if !errors.Is(err, storageFailure) {
			t.Fatalf("storage failure was hidden: %v", err)
		}
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
