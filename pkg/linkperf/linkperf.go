// Package linkperf provides HTTP endpoints for browser/client latency and bandwidth probes.
package linkperf

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	// PingPath is GET-only JSON RTT anchor (client measures round-trip).
	PingPath = "/link/ping"
	// BenchPath supports GET (download) and POST (upload) throughput tests.
	BenchPath = "/link/bench"
)

// Options limits probe payload sizes.
type Options struct {
	MaxBytes int64
}

// DefaultOptions returns safe defaults for public APIs.
func DefaultOptions() Options {
	return Options{MaxBytes: 512 << 10} // 512 KiB
}

// Register mounts linkperf routes on mux.
func Register(mux *http.ServeMux, opt Options) {
	if opt.MaxBytes <= 0 {
		opt = DefaultOptions()
	}
	h := &handler{opt: opt}
	mux.HandleFunc("GET "+PingPath, h.ping)
	mux.HandleFunc("GET "+BenchPath, h.benchDown)
	mux.HandleFunc("POST "+BenchPath, h.benchUp)
}

type handler struct {
	opt Options
}

func (h *handler) ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int64{"t": time.Now().UnixMilli()})
}

func (h *handler) benchDown(w http.ResponseWriter, r *http.Request) {
	n := parseBytes(r.URL.Query().Get("bytes"), 128<<10, h.opt.MaxBytes)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Cogs-Bytes", strconv.FormatInt(n, 10))
	w.WriteHeader(http.StatusOK)
	chunk := make([]byte, 32<<10)
	var sent int64
	for sent < n {
		toWrite := int64(len(chunk))
		if remain := n - sent; remain < toWrite {
			toWrite = remain
		}
		if _, err := w.Write(chunk[:toWrite]); err != nil {
			return
		}
		sent += toWrite
	}
}

func (h *handler) benchUp(w http.ResponseWriter, r *http.Request) {
	n := parseBytes(r.URL.Query().Get("bytes"), 64<<10, h.opt.MaxBytes)
	w.Header().Set("Cache-Control", "no-store")
	body := io.LimitReader(r.Body, n)
	read, err := io.Copy(io.Discard, body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int64{"received": read})
}

func parseBytes(raw string, def, max int64) int64 {
	if raw == "" {
		return def
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
