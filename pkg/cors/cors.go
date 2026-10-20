package cors

import (
	"net/http"
	"strings"
)

// Middleware allows browser calls from listed origins (Pages → tunnel API).
func Middleware(allowedOrigins []string) func(http.Handler) http.Handler {
	set := map[string]struct{}{}
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o != "" {
			set[o] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				_, star := set["*"]
				_, ok := set[origin]
				if ok || star {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Client-Id")
					w.Header().Set("Access-Control-Max-Age", "86400")
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
