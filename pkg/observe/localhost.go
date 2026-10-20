package observe

import (
	"net"
	"net/http"
	"strings"
)

// LocalAdminHost reports whether the request Host is loopback (not the public tunnel hostname).
func LocalAdminHost(r *http.Request) bool {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

// LocalAdminOnly hides /admin from requests that arrive via the public Host (e.g. tunnel).
func LocalAdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !LocalAdminHost(r) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
