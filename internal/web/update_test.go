package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"shellyficator/internal/domain"
)

type updateActions struct {
	Devices
	selections []domain.FirmwareSelection
	calls      int
}

func (a *updateActions) StartFirmwareUpdates(_ context.Context, selections []domain.FirmwareSelection) error {
	a.calls++
	a.selections = selections
	return nil
}

func TestFirmwareInstallRequiresConfirmationAndCarriesExactVersions(t *testing.T) {
	want := []domain.FirmwareSelection{
		{DeviceID: "one", Stage: "stable", FromVersion: "2.1.0", TargetVersion: "2.2.0"},
		{DeviceID: "two", Stage: "beta", FromVersion: "2.1.1", TargetVersion: "2.3.0-beta1"},
	}
	for _, test := range []struct{ confirmed, ajax bool }{{false, false}, {true, false}, {true, true}} {
		actions := &updateActions{}
		s := &Server{devices: actions}
		form := url.Values{"device": {"one", "two"}, "stage_one": {"stable"}, "from_one": {"2.1.0"}, "target_stable_one": {"2.2.0"}, "stage_two": {"beta"}, "from_two": {"2.1.1"}, "target_beta_two": {"2.3.0-beta1"}}
		if test.confirmed {
			form.Set("confirm", "update")
		}
		req := httptest.NewRequest(http.MethodPost, "/firmware/update", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if test.ajax {
			req.Header.Set("HX-Request", "true")
		}
		response := httptest.NewRecorder()
		event := &core.RequestEvent{Event: router.Event{Request: req, Response: response}}
		err := s.updateFirmware(event)
		if !test.confirmed {
			if err == nil || actions.calls != 0 {
				t.Fatal("update started without confirmation")
			}
			continue
		}
		if err != nil || actions.calls != 1 || !slices.Equal(actions.selections, want) {
			t.Fatalf("confirmed batch failed: %v %d %+v", err, response.Code, actions)
		}
		if test.ajax {
			if response.Code != http.StatusNoContent || !strings.Contains(response.Header().Get("HX-Trigger"), "Firmware updates queued.") || response.Body.Len() != 0 {
				t.Fatal("accepted update lost its toast or attempted to replace the confirmation")
			}
		} else if response.Code != http.StatusSeeOther || !strings.HasPrefix(response.Header().Get("Location"), "/?notice=") {
			t.Fatal("native confirmation lost its redirect")
		}
	}
}

func TestFirmwareResultPresentation(t *testing.T) {
	for _, test := range []struct {
		phase, label, tone string
		active             bool
	}{
		{"queued", "Update queued", "blue", true},
		{"capturing", "Capturing settings", "blue", true},
		{"verifying", "Updating · verifying", "blue", true},
		{"succeeded", "Updated", "ok", false},
		{"failed", "Update failed", "error", false},
		{"unconfirmed", "Update not verified", "warning", false},
		{"warning", "Updated · capture unavailable", "warning", false},
	} {
		device := domain.Device{Firmware: "2.2.0", FirmwareUpdate: &domain.FirmwareUpdate{ID: "attempt", Phase: test.phase, FromVersion: "2.1.0", TargetVersion: "2.2.0", Message: "Result detail"}}
		view := firmwareView(device)
		if view.Active != test.active || view.Label != test.label || view.Tone != test.tone || !view.Visible || !strings.Contains(view.Info, "Result detail") {
			t.Fatalf("incorrect progress/result for %s: %+v", test.phase, view)
		}
	}
}

func TestCompletedFirmwareAttemptDoesNotHideANewerOffer(t *testing.T) {
	device := domain.Device{Firmware: "2.2.0", Reachable: true, StatusAvailable: true,
		Status:         map[string]any{"sys": map[string]any{"available_updates": map[string]any{"stable": map[string]any{"version": "2.3.0"}}}},
		FirmwareUpdate: &domain.FirmwareUpdate{Phase: "succeeded", FromVersion: "2.1.0", TargetVersion: "2.2.0", Message: "Updated to 2.2.0"}}
	view := firmwareView(device)
	if view.Label != "Update available" || view.Tone != "warning" || !strings.Contains(view.Info, "2.3.0") {
		t.Fatalf("completed update hid the newer offer: %+v", view)
	}
}
