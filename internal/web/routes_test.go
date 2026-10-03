package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func TestRoutesPreserveAccessAndDeviceActions(t *testing.T) {
	var stopping atomic.Bool
	started, cancelled := make(chan struct{}), make(chan struct{})
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shelly":
			if stopping.Load() {
				close(started)
				<-r.Context().Done()
				close(cancelled)
				return
			}
			_, _ = w.Write([]byte(`{"id":"test-device","mac":"AABBCCDDEEFF","gen":4}`))
		case "/rpc/Shelly.GetStatus":
			_, _ = w.Write([]byte(`{"switch:0":{"output":true}}`))
		case "/rpc/Shelly.GetConfig":
			_, _ = w.Write([]byte(`{"switch:0":{"in_mode":"detached"}}`))
		default:
			t.Errorf("unexpected device request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	r, err := apis.NewRouter(s.store.App)
	if err != nil {
		t.Fatal(err)
	}
	s.store.App.OnTerminate().BindFunc(func(event *core.TerminateEvent) error {
		s.devices.Close()
		return event.Next()
	})
	if err := Register(&core.ServeEvent{App: s.store.App, Router: r}, s.devices, s.accounts, "test"); err != nil {
		t.Fatal(err)
	}
	mux, err := r.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	cookie := func(response *httptest.ResponseRecorder, name string) *http.Cookie {
		t.Helper()
		for _, cookie := range response.Result().Cookies() {
			if cookie.Name == name {
				return cookie
			}
		}
		t.Fatalf("missing %s cookie", name)
		return nil
	}
	for path, status := range map[string]int{"/healthz": 200, "/api/health": 200, "/favicon.ico": 204, "/assets/app.js": 200, "/login": 303, "/setup": 200} {
		if response := request(http.MethodGet, path, nil); response.Code != status {
			t.Fatalf("public GET %s: got %d, want %d", path, response.Code, status)
		}
	}
	for _, path := range []string{"/setup", "/login"} {
		if response := request(http.MethodPost, path, nil); response.Code != http.StatusForbidden {
			t.Fatalf("public POST %s bypassed CSRF: %d", path, response.Code)
		}
	}
	csrfCookie := cookie(request(http.MethodGet, "/setup", nil), "shelly_csrf")
	credentials := url.Values{"email": {"operator@example.com"}, "password": {"routing-test-password"}, "csrf": {csrfCookie.Value}}
	if response := request(http.MethodPost, "/setup", credentials, csrfCookie); response.Code != http.StatusSeeOther || !strings.HasPrefix(response.Header().Get("Location"), "/login?") {
		t.Fatalf("valid setup failed: %d %s", response.Code, response.Header().Get("Location"))
	}
	login := request(http.MethodPost, "/login", credentials, csrfCookie)
	if login.Code != http.StatusSeeOther || login.Header().Get("Location") != "/" {
		t.Fatalf("valid login failed: %d %s", login.Code, login.Header().Get("Location"))
	}
	session := cookie(login, "shelly_session")
	if response := request(http.MethodPost, "/api/collections/operators/auth-with-password", url.Values{"identity": {credentials.Get("email")}, "password": {credentials.Get("password")}}); response.Code != http.StatusOK {
		t.Fatalf("application middleware affected PocketBase authentication: %d", response.Code)
	}
	devicePath := "/devices/" + id
	for _, path := range []string{"/", "/inventory", "/inventory/refresh", "/discovery/progress", devicePath, devicePath + "/organization"} {
		if response := request(http.MethodGet, path, nil); response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
			t.Fatalf("GET %s bypassed authentication: %d", path, response.Code)
		}
		if response := request(http.MethodGet, path, nil, session); response.Code != http.StatusOK {
			t.Fatalf("authenticated GET %s required CSRF or failed: %d", path, response.Code)
		}
	}
	hxRequest := httptest.NewRequest(http.MethodGet, devicePath, nil)
	hxRequest.Header.Set("HX-Request", "true")
	hxResponse := httptest.NewRecorder()
	mux.ServeHTTP(hxResponse, hxRequest)
	if hxResponse.Code != http.StatusUnauthorized || hxResponse.Header().Get("HX-Redirect") != "/login" {
		t.Fatalf("HTMX authentication redirect lost: %d %v", hxResponse.Code, hxResponse.Header())
	}
	for _, path := range []string{"/logout", "/discover", devicePath + "/refresh", devicePath + "/organization", devicePath + "/metadata", devicePath + "/channel-notes", devicePath + "/capture", devicePath + "/baseline"} {
		if response := request(http.MethodPost, path, nil); response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
			t.Fatalf("POST %s lost authentication precedence: %d", path, response.Code)
		}
		if response := request(http.MethodPost, path, nil, session); response.Code != http.StatusForbidden {
			t.Fatalf("POST %s bypassed CSRF: %d", path, response.Code)
		}
	}
	for _, action := range []struct {
		path, target, notice string
		form                 url.Values
	}{
		{"/refresh", devicePath, "Status refreshed.", url.Values{}},
		{"/organization", "/", "Room & tags saved.", url.Values{"room": {"Dining"}, "tags": {"lighting"}}},
		{"/metadata", devicePath, "Device info saved.", url.Values{"room": {"Dining"}, "tags": {"lighting"}, "notes": {"Device note"}}},
		{"/channel-notes", devicePath, "Note saved.", url.Values{"component": {"switch:0"}, "notes": {"Channel note"}}},
		{"/capture", devicePath, "Configuration captured.", url.Values{}},
		{"/baseline", devicePath, "Baseline updated.", url.Values{}},
	} {
		action.form.Set("csrf", csrfCookie.Value)
		location := action.target + "?notice=" + url.QueryEscape(action.notice)
		if response := request(http.MethodPost, devicePath+action.path, action.form, session, csrfCookie); response.Code != http.StatusSeeOther || response.Header().Get("Location") != location {
			t.Fatalf("valid POST %s: got %d %s, want 303 %s", action.path, response.Code, response.Header().Get("Location"), location)
		}
	}
	record, err := s.record(id)
	if err != nil {
		t.Fatal(err)
	}
	if record.GetString("room") != "Dining" || record.GetString("notes") != "Device note" || s.channelNotes(id)["switch:0"] != "Channel note" || record.GetString("baseline") == "" {
		t.Fatal("device actions did not receive their path/form values")
	}
	for _, path := range []string{"/unknown", devicePath + "/unknown"} {
		if response := request(http.MethodGet, path, nil, session); response.Code != http.StatusNotFound {
			t.Fatalf("unknown path %s matched an application page: %d", path, response.Code)
		}
	}
	if response := request(http.MethodPost, "/logout", url.Values{"csrf": {csrfCookie.Value}}, session, csrfCookie); response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatalf("valid logout failed: %d", response.Code)
	}
	// The registered service drains before termination reaches storage teardown.
	stopping.Store(true)
	finished := make(chan struct{})
	go func() {
		request(http.MethodGet, "/inventory/refresh?refresh=1", nil, session)
		close(finished)
	}()
	waitForRefresh(t, started)
	if err := s.store.App.OnTerminate().Trigger(&core.TerminateEvent{App: s.store.App}, func(_ *core.TerminateEvent) error {
		waitForRefresh(t, cancelled)
		waitForRefresh(t, finished)
		record, err := s.record(id)
		if err != nil || record.GetInt("consecutive_failures") != 0 || !record.GetBool("reachable") {
			t.Fatal("termination recorded a device failure or closed storage before draining jobs")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
