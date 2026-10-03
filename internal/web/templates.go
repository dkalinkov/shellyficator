package web

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"shellyficator/internal/inventory"
)

//go:embed templates/*.html static/*
var assets embed.FS

func parseTemplates() (*template.Template, error) {
	functions := template.FuncMap{
		"get":   func(r *core.Record, key string) string { return r.GetString(key) },
		"flag":  func(r *core.Record, key string) bool { return r.GetBool(key) },
		"when":  inventory.FormatTime,
		"value": inventory.Display,
		"tags":  inventory.SplitTags,
	}
	return template.New("app").Funcs(functions).ParseFS(assets, "templates/*.html")
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
