package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestDetailNotificationsSurviveHTMXSelectionAndEscapeContent(t *testing.T) {
	s, id := refreshTestServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	message := `Saved <script>alert("test")</script> & "quoted" text.`
	for _, test := range []struct {
		name, level, wantLevel string
		htmx                   bool
	}{
		{"ordinary page", "", "success", false},
		{"HTMX confirmation", "", "success", true},
		{"HTMX error", "error", "error", true},
		{"HTMX warning", "warning", "warning", true},
		{"unknown level", "untrusted", "success", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			event, response := detailEvent(s, id, http.MethodGet, "", nil)
			event.Request.URL.RawQuery = url.Values{"notice": {message}, "notice_level": {test.level}}.Encode()
			if test.htmx {
				event.Request.Header.Set("HX-Request", "true")
			}
			if err := s.detail(event); err != nil {
				t.Fatal(err)
			}
			body := response.Body.String()
			if strings.Contains(body, message) || strings.Count(body, "data-initial-notice") != 1 || !strings.Contains(body, `data-level="`+test.wantLevel+`"`) {
				t.Fatal("initial notification lost its escaping, level, or single rendering")
			}
			header := response.Header().Get("HX-Trigger")
			if !test.htmx {
				if header != "" {
					t.Fatal("ordinary page should only show its rendered notification")
				}
				return
			}
			var trigger map[string]map[string]string
			if err := json.Unmarshal([]byte(header), &trigger); err != nil {
				t.Fatal(err)
			}
			toast := trigger["notify"]
			if toast["message"] != message || toast["level"] != test.wantLevel || toast["target"] != "#toast-stack" {
				t.Fatalf("notification would be lost when HTMX selects only the detail sections: %v", toast)
			}
		})
	}
}
