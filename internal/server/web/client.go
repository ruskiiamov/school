package web

import (
	"net"
	"net/http"
	"strings"
)

func (b *Base) ClientIP(r *http.Request) string {
	if b.behindProxy {
		if ip := lastForwardedFor(r); ip != "" {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}

	return host
}

func lastForwardedFor(r *http.Request) string {
	values := r.Header.Values("X-Forwarded-For")
	if len(values) == 0 {
		return ""
	}

	parts := strings.Split(values[len(values)-1], ",")
	candidate := strings.TrimSpace(parts[len(parts)-1])

	if net.ParseIP(candidate) == nil {
		return ""
	}

	return candidate
}
