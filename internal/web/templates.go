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
		"base":       func() string { return "" },
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
	query := e.Request.URL.Query()
	level := query.Get("notice_level")
	if level != "error" && level != "warning" {
		level = "success"
	}
	return PageMeta{Title: title, CSRF: s.csrf(e), Notice: query.Get("notice"), NoticeLevel: level, Version: s.version, HomeAssistant: s.homeAssistant}
}

func (s *Server) render(e *core.RequestEvent, name string, data any) error {
	templates, err := s.templatesFor(s.base(e))
	if err != nil {
		return err
	}
	var buffer bytes.Buffer
	if err := templates.ExecuteTemplate(&buffer, name, data); err != nil {
		return err
	}
	return e.HTML(http.StatusOK, buffer.String())
}

// Template functions are shared by all executions of a template set, so each
// base gets its own clone of the master, which therefore must never execute.
func (s *Server) templatesFor(base string) (*template.Template, error) {
	s.templatesMu.Lock()
	defer s.templatesMu.Unlock()
	if templates, ok := s.baseTemplates[base]; ok {
		return templates, nil
	}
	templates, err := s.templates.Clone()
	if err != nil {
		return nil, err
	}
	templates.Funcs(template.FuncMap{"base": func() string { return base }})
	if s.baseTemplates == nil {
		s.baseTemplates = map[string]*template.Template{}
	}
	s.baseTemplates[base] = templates
	return templates, nil
}
