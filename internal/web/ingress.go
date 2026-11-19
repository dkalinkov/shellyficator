package web

import (
	"net"
	"regexp"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

const supervisorAddress = "172.30.32.2"

var ingressPath = regexp.MustCompile(`^/api/hassio_ingress/[A-Za-z0-9_-]+$`)

func ingressOnly(e *core.RequestEvent) error {
	// Home Assistant signs users in, so only its proxy may connect. The TCP peer
	// is checked because forwarded headers, which RealIP may trust, come from clients.
	if host, _, _ := net.SplitHostPort(e.Request.RemoteAddr); host != supervisorAddress {
		return e.ForbiddenError("", nil)
	}
	if path := e.Request.URL.Path; strings.HasPrefix(path, "/_/") || strings.HasPrefix(path, "/api/") {
		return e.NotFoundError("", nil)
	}
	return e.Next()
}

func (s *Server) base(e *core.RequestEvent) string {
	if path := e.Request.Header.Get("X-Ingress-Path"); s.homeAssistant && ingressPath.MatchString(path) {
		return path
	}
	return ""
}
