package ingress

import (
	"net/http"
)

// Middleware enforces tunnel-first access unless a route sets allow_direct in cogs.ingress.json.
func Middleware(cfg *Config, next http.Handler) http.Handler {
	if cfg == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.directAllowed(r) {
			next.ServeHTTP(w, r)
			return
		}
		if cfg.tunnelAllowed(r) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "ingress: route requires Cloudflare Tunnel (or allow_direct in cogs.ingress.json)", http.StatusForbidden)
	})
}

// Wrap returns handler with ingress middleware when cfg is non-nil.
func Wrap(cfg *Config, h http.Handler) http.Handler {
	return Middleware(cfg, h)
}
