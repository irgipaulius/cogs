package observe

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/irgipaulius/cogs/pkg/config"
)

// Runtime is the opinionated CoGS observability bundle.
type Runtime struct {
	Mode   Mode
	Logger *slog.Logger
	Hub    *Hub
	HTTP   *HTTPStats
	SQL    *SQLStats
	logFile *os.File
}

// HTTPStats counts requests for the admin dashboard.
type HTTPStats struct {
	Total    atomic.Uint64
	InFlight atomic.Int64
	LastMS   atomic.Uint64
}

// SQLStats counts database operations when using InstrumentDB.
type SQLStats struct {
	Queries atomic.Uint64
	Execs   atomic.Uint64
}

// Config from app env.
type Config struct {
	Mode    Mode
	LogPath string
	DBPath  string
}

// Setup builds logger + stats from COGS_OBS / COGS_OBS_LOG.
func Setup(cfg config.App) (*Runtime, error) {
	c := Config{
		Mode:    ParseMode(cfg.ObsMode),
		LogPath: cfg.ObsLogPath,
		DBPath:  cfg.DBPath,
	}
	return setup(c)
}

func setup(c Config) (*Runtime, error) {
	if c.Mode == ModeOff {
		return &Runtime{Mode: ModeOff, Logger: slog.Default()}, nil
	}

	hub := NewHub(defaultRingCap)
	var writers []io.Writer
	writers = append(writers, os.Stderr)

	var logFile *os.File
	if c.LogPath != "" {
		if err := os.MkdirAll(filepath.Dir(c.LogPath), 0o750); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(c.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		logFile = f
		writers = append(writers, f)
	}

	mw := io.MultiWriter(writers...)
	handler := slog.NewTextHandler(mw, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	logger := slog.New(&hubHandler{inner: handler, hub: hub})

	return &Runtime{
		Mode:    c.Mode,
		Logger:  logger,
		Hub:     hub,
		HTTP:    &HTTPStats{},
		SQL:     &SQLStats{},
		logFile: logFile,
	}, nil
}

type hubHandler struct {
	inner slog.Handler
	hub   *Hub
}

func (h *hubHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *hubHandler) Handle(ctx context.Context, r slog.Record) error {
	h.hub.Append(r.Level.String(), r.Message)
	return h.inner.Handle(ctx, r)
}

func (h *hubHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &hubHandler{inner: h.inner.WithAttrs(attrs), hub: h.hub}
}

func (h *hubHandler) WithGroup(name string) slog.Handler {
	return &hubHandler{inner: h.inner.WithGroup(name), hub: h.hub}
}

// Close releases the log file handle.
func (r *Runtime) Close() error {
	if r.logFile != nil {
		return r.logFile.Close()
	}
	return nil
}

// AdminEnabled is true when the local /admin UI is mounted (file or dashboard mode).
func (r *Runtime) AdminEnabled() bool {
	return r != nil && r.Mode != ModeOff
}

// DashboardEnabled is an alias for AdminEnabled.
func (r *Runtime) DashboardEnabled() bool {
	return r.AdminEnabled()
}

// RecordRequest is called from access-log middleware.
func (r *Runtime) RecordRequest(method, path string, status int, dur time.Duration) {
	if r == nil || r.Mode == ModeOff {
		return
	}
	r.HTTP.Total.Add(1)
	r.HTTP.LastMS.Store(uint64(dur.Milliseconds()))
	if r.Logger != nil {
		r.Logger.Info("incoming request", "method", method, "path", path, "status", status, "ms", dur.Milliseconds())
	}
}
