package observe

import (
	"net/http"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

// Middleware logs each HTTP request when observability is enabled.
func (r *Runtime) Middleware(next http.Handler) http.Handler {
	if r == nil || r.Mode == ModeOff {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/admin" || len(req.URL.Path) >= 7 && req.URL.Path[:7] == "/admin/" {
			next.ServeHTTP(w, req)
			return
		}
		if req.URL.Path == "/link/bench" {
			next.ServeHTTP(w, req)
			return
		}
		r.HTTP.InFlight.Add(1)
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(sw, req)
		r.HTTP.InFlight.Add(-1)
		r.RecordRequest(req.Method, req.URL.Path, sw.code, time.Since(start))
	})
}
