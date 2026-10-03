package web

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"

	"github.com/pocketbase/pocketbase/core"
)

//go:embed templates/*.html static/*
var assets embed.FS

func parseTemplates() (*template.Template, error) {
	functions := template.FuncMap{
		"toneClass":  toneClass,
		"badgeClass": badgeClass,
	}
	return template.New("app").Funcs(functions).ParseFS(assets, "templates/*.html")
}

// Complete class names let Tailwind discover styles for data-driven tones.
var toneStyles = map[string]struct{ text, badge string }{
	"ok":      {"text-success", "badge-success"},
	"warning": {"text-warning", "badge-warning"},
	"error":   {"text-error", "badge-error"},
	"blue":    {"text-info", "badge-info"},
	"violet":  {"text-secondary", "badge-secondary"},
	"pink":    {"text-accent", "badge-accent"},
	"muted":   {"text-base-content/60", "badge-ghost text-base-content/60"},
}

func toneClass(tone string) string {
	if style, ok := toneStyles[tone]; ok {
		return style.text
	}
	return toneStyles["muted"].text
}

func badgeClass(tone string) string {
	if style, ok := toneStyles[tone]; ok {
		return style.badge
	}
	return toneStyles["muted"].badge
}

func (s *Server) pageMeta(e *core.RequestEvent, title string) PageMeta {
	return PageMeta{Title: title, CSRF: csrf(e), Notice: e.Request.URL.Query().Get("notice"), Version: s.version}
}

func (s *Server) render(e *core.RequestEvent, name string, data any) error {
	var buffer bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buffer, name, data); err != nil {
		return err
	}
	return e.HTML(http.StatusOK, buffer.String())
}
