// Package server builds the CoGS HTTP mux with standard middleware and routes.
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/irgipaulius/cogs/pkg/accessjwt"
	"github.com/irgipaulius/cogs/pkg/cockpit"
	"github.com/irgipaulius/cogs/pkg/config"
	"github.com/irgipaulius/cogs/pkg/ingress"
	"github.com/irgipaulius/cogs/pkg/limiter"
	"github.com/irgipaulius/cogs/pkg/linkperf"
	"github.com/irgipaulius/cogs/pkg/observe"
	"github.com/irgipaulius/cogs/pkg/session"
)

// Deps wires handlers and middleware.
type Deps struct {
	Config         config.App
	DB             *sql.DB
	Session        *session.Manager
	Access         *accessjwt.Validator
	IPLimiter      *limiter.LRU
	TokenLimiter   *limiter.LRU
	MigrationVer   int64
	Metrics        http.Handler
	PrivateOnly    bool
	Observe        *observe.Runtime
	Cockpit        *cockpit.Telemetry
	Ingress        *ingress.Config
	RegisterRoutes func(mux *http.ServeMux)
}

// NewMux returns the application handler chain.
func NewMux(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		var one int
		if err := d.DB.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
			http.Error(w, "db unhealthy", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":                true,
			"migration_version": d.MigrationVer,
		})
	})
	if d.Metrics != nil {
		mux.Handle("GET /metrics", d.Metrics)
	}
	if d.Config.LinkPerf {
		linkperf.Register(mux, linkperf.Options{MaxBytes: d.Config.LinkPerfMaxBytes})
	}
	mux.HandleFunc("POST /session", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		ip := clientIP(r)
		if err := d.Session.VerifyTurnstile(r.Context(), body.Token, ip); err != nil {
			http.Error(w, "turnstile failed", http.StatusForbidden)
			return
		}
		tok, err := d.Session.Mint(ip, time.Now())
		if err != nil {
			http.Error(w, "session error", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": tok})
	})
	if d.RegisterRoutes != nil {
		d.RegisterRoutes(mux)
	}
	if d.Observe != nil {
		d.Observe.RegisterAdminRoutes(mux, d.Config.DBPath, d.DB)
	}
	if d.Cockpit != nil {
		d.Cockpit.RegisterWellKnown(mux)
	}

	var h http.Handler = mux
	if d.Cockpit != nil {
		h = d.Cockpit.Middleware(h)
	}
	if d.Observe != nil {
		h = d.Observe.Middleware(h)
	}
	if d.IPLimiter != nil {
		h = limiter.Middleware(d.IPLimiter, "CF-Connecting-IP")(h)
	}
	if d.PrivateOnly && d.Config.PrivateToken != "" {
		h = privateTokenMiddleware(d.Config.PrivateToken)(h)
	}
	if d.Ingress != nil {
		h = ingress.Middleware(d.Ingress, h)
	}
	return h
}

func privateTokenMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" || r.URL.Path == linkperf.PingPath || r.URL.Path == linkperf.BenchPath {
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get("X-Cogs-Private-Token") != token {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

// ListenAndServe starts the HTTP server with timeouts and graceful shutdown hook.
func ListenAndServe(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
}
