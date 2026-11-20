package web

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"shellyficator/internal/domain"
)

type failingWatchStore struct{ err error }

func (s failingWatchStore) WatchPreferences() (domain.WatchPreferences, error) { return nil, s.err }
func (s failingWatchStore) SaveWatchPolicy(string, domain.WatchPolicy) error   { return s.err }

func TestWatchReadFailureDoesNotFallBackToDefaults(t *testing.T) {
	failure := errors.New("preferences unavailable")
	server := &Server{watchStore: failingWatchStore{failure}}
	event := &core.RequestEvent{Event: router.Event{Request: httptest.NewRequest("GET", "/settings", nil), Response: httptest.NewRecorder()}}
	if err := server.settingsPage(event); !errors.Is(err, failure) {
		t.Fatalf("read failure was hidden: %v", err)
	}
}
