package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

const testIngress = "/api/hassio_ingress/Token_123-abc"

func homeAssistantRouter(t *testing.T) (http.Handler, string) {
	t.Helper()
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
	r, err := apis.NewRouter(s.store.App)
	if err != nil {
		t.Fatal(err)
	}
	// apis.Serve adds CORS and the admin UI outside NewRouter.
	r.Bind(apis.CORS(apis.CORSConfig{AllowOrigins: []string{"*"}}))
	r.GET("/_/{path...}", func(e *core.RequestEvent) error { return e.String(http.StatusOK, "admin") })
	if err := Register(&core.ServeEvent{App: s.store.App, Router: r}, s.devices, s.accounts, Options{HomeAssistant: true, WatchStore: s.store}); err != nil {
		t.Fatal(err)
	}
	mux, err := r.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	return mux, id
}

func fromSupervisor(method, path string, form url.Values, cookies ...*http.Cookie) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.RemoteAddr = supervisorAddress + ":41234"
	req.Header.Set("X-Ingress-Path", testIngress)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	return req
}

func serve(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestHomeAssistantServesOnlyTheSupervisor(t *testing.T) {
	mux, _ := homeAssistantRouter(t)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/healthz"}, {http.MethodGet, "/"}, {http.MethodGet, "/api/health"}, {http.MethodGet, "/missing"}, {http.MethodOptions, "/api/health"},
	} {
		req := fromSupervisor(route.method, route.path, nil)
		req.RemoteAddr = "172.30.32.1:41234"
		req.Header.Set("X-Forwarded-For", supervisorAddress)
		req.Header.Set("X-Real-IP", supervisorAddress)
		req.Header.Set("Origin", "http://example.com")
		if response := serve(mux, req); response.Code != http.StatusForbidden {
			t.Fatalf("%s %s from another peer: got %d, want 403", route.method, route.path, response.Code)
		}
	}
	if response := serve(mux, fromSupervisor(http.MethodGet, "/healthz", nil)); response.Code != http.StatusOK {
		t.Fatalf("supervisor health check: got %d", response.Code)
	}
}

func TestHomeAssistantHidesPocketBase(t *testing.T) {
	mux, _ := homeAssistantRouter(t)
	for _, path := range []string{"/_/", "/api/health"} {
		if response := serve(mux, fromSupervisor(http.MethodGet, path, nil)); response.Code != http.StatusNotFound {
			t.Fatalf("GET %s: got %d, want 404", path, response.Code)
		}
	}
}

func TestHomeAssistantServesDashboardWithoutLogin(t *testing.T) {
	mux, _ := homeAssistantRouter(t)
	if response := serve(mux, fromSupervisor(http.MethodGet, "/", nil)); response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("dashboard without a session: got %d %v", response.Code, response.Header())
	}
	for _, route := range []struct{ method, path string }{{http.MethodGet, "/login"}, {http.MethodGet, "/setup"}, {http.MethodPost, "/logout"}} {
		if response := serve(mux, fromSupervisor(route.method, route.path, nil)); response.Code != http.StatusNotFound {
			t.Fatalf("%s %s: got %d, want 404", route.method, route.path, response.Code)
		}
	}
}

func TestHomeAssistantURLsCarryTheIngressPath(t *testing.T) {
	mux, id := homeAssistantRouter(t)
	page := serve(mux, fromSupervisor(http.MethodGet, "/", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("dashboard returned %d", page.Code)
	}
	for _, match := range regexp.MustCompile(`="(/[^"]*)"`).FindAllStringSubmatch(page.Body.String(), -1) {
		if match[1] != testIngress && !strings.HasPrefix(match[1], testIngress+"/") {
			t.Fatalf("dashboard URL without the ingress path: %s", match[1])
		}
	}
	if body := page.Body.String(); strings.Contains(body, "/_/") || strings.Contains(body, "/logout") {
		t.Fatal("dashboard links to the data admin or sign out")
	}
	var csrf *http.Cookie
	for _, cookie := range page.Result().Cookies() {
		if cookie.Name == "shelly_csrf" {
			csrf = cookie
		}
	}
	if csrf == nil || csrf.Path != testIngress+"/" {
		t.Fatalf("csrf cookie: %+v", csrf)
	}
	form := url.Values{"csrf": {csrf.Value}, "room": {"Dining"}}
	location := testIngress + "/devices/" + id + "?notice=" + url.QueryEscape("Device info saved.")
	if response := serve(mux, fromSupervisor(http.MethodPost, "/devices/"+id+"/metadata", form, csrf)); response.Code != http.StatusSeeOther || response.Header().Get("Location") != location {
		t.Fatalf("redirect: got %d %q, want %q", response.Code, response.Header().Get("Location"), location)
	}
	if got := serve(mux, fromSupervisor(http.MethodGet, "/inventory?q=lamp", nil)).Header().Get("HX-Replace-Url"); got != testIngress+"/?q=lamp" {
		t.Fatalf("HX-Replace-Url: got %q", got)
	}
}

func TestIngressPathIsResolvedForEachRequest(t *testing.T) {
	mux, _ := homeAssistantRouter(t)
	for _, test := range []struct{ header, prefix string }{
		{testIngress, testIngress},
		{"/api/hassio_ingress/other", "/api/hassio_ingress/other"},
		{testIngress + "/extra", ""},
		{"/api/hassio_ingress/", ""},
		{"//example.test" + testIngress, ""},
		{testIngress + `"><invalid>`, ""},
		{testIngress, testIngress},
	} {
		req := fromSupervisor(http.MethodGet, "/", nil)
		req.Header.Set("X-Ingress-Path", test.header)
		response := serve(mux, req)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `src="`+test.prefix+`/assets/app.js"`) {
			t.Errorf("dashboard did not use prefix %q for header %q", test.prefix, test.header)
		}
	}
}
