package cockpit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const defaultRecent = 400

// RequestLine is one access log row.
type RequestLine struct {
	At       time.Time `json:"at"`
	IP       string    `json:"ip"`
	Country  string    `json:"country,omitempty"`
	Internal bool      `json:"internal,omitempty"`
	Method   string    `json:"method"`
	Path     string    `json:"path"`
	Body     string    `json:"body,omitempty"`
	Status   int       `json:"status"`
	MS       int64     `json:"ms"`
}

// Snapshot is exported telemetry for admin / supervisor.
type Snapshot struct {
	App            string        `json:"app"`
	UpdatedAt      time.Time     `json:"updated_at"`
	RequestsTotal  uint64        `json:"requests_total"`
	InFlight       int64         `json:"in_flight"`
	PeakInFlight   int64         `json:"peak_in_flight"`
	UniqueIPs        int            `json:"unique_ips_session"`
	ByCountry        map[string]int `json:"by_country,omitempty"`
	ByCountryRead    map[string]int `json:"by_country_read,omitempty"`
	ByCountryWrite   map[string]int `json:"by_country_write,omitempty"`
	Recent           []RequestLine  `json:"recent"`
	Listen         string        `json:"listen,omitempty"`
	PublicURL      string        `json:"public_url,omitempty"`
}

// Telemetry collects HTTP stats for any CoGS app (always-on middleware).
type Telemetry struct {
	App       string
	Listen    string
	PublicURL string
	Path      string

	total      atomic.Uint64
	inFlight   atomic.Int64
	peakFlight atomic.Int64
	mu         sync.Mutex
	uniqueIPs  map[string]struct{}
	byCountry      map[string]int
	byCountryRead  map[string]int
	byCountryWrite map[string]int
	recent         []RequestLine
}

// New builds telemetry and starts periodic flush to Path.
func New(appName, listen, publicURL, persistPath string) *Telemetry {
	t := &Telemetry{
		App:       appName,
		Listen:    listen,
		PublicURL: publicURL,
		Path:      persistPath,
		uniqueIPs: map[string]struct{}{},
		byCountry:      map[string]int{},
		byCountryRead:  map[string]int{},
		byCountryWrite: map[string]int{},
	}
	if t.Path != "" {
		go t.flushLoop()
	}
	return t
}

func (t *Telemetry) flushLoop() {
	tick := time.NewTicker(time.Second)
	for range tick.C {
		_ = t.Persist()
	}
}

// Persist writes snapshot JSON for supervisor scraping.
func (t *Telemetry) Persist() error {
	if t.Path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(t.Path), 0o750); err != nil {
		return err
	}
	s := t.Snapshot()
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(t.Path, b, 0o644)
}

// Snapshot returns a point-in-time copy.
func (t *Telemetry) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	rec := append([]RequestLine(nil), t.recent...)
	copyMap := func(m map[string]int) map[string]int {
		out := make(map[string]int, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out
	}
	return Snapshot{
		App:           t.App,
		UpdatedAt:     time.Now().UTC(),
		RequestsTotal: t.total.Load(),
		InFlight:      t.inFlight.Load(),
		PeakInFlight:  t.peakFlight.Load(),
		UniqueIPs:     len(t.uniqueIPs),
		ByCountry:     copyMap(t.byCountry),
		ByCountryRead: copyMap(t.byCountryRead),
		ByCountryWrite: copyMap(t.byCountryWrite),
		Recent:        rec,
		Listen:        t.Listen,
		PublicURL:     t.PublicURL,
	}
}

func (t *Telemetry) noteRequest(line RequestLine) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if line.IP != "" && !line.Internal {
		t.uniqueIPs[line.IP] = struct{}{}
	}
	if line.Country != "" {
		t.byCountry[line.Country]++
		if isWriteMethod(line.Method) {
			t.byCountryWrite[line.Country]++
		} else if isReadMethod(line.Method) {
			t.byCountryRead[line.Country]++
		}
	}
	if shouldKeepRecent(line) {
		t.recent = append(t.recent, line)
		if len(t.recent) > defaultRecent {
			t.recent = t.recent[len(t.recent)-defaultRecent:]
		}
	}
	t.appendAccessLog(line)
}

func isReadMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead:
		return true
	default:
		return false
	}
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func shouldKeepRecent(line RequestLine) bool {
	if line.Method == http.MethodOptions {
		return false
	}
	switch line.Path {
	case "/health", "/.well-known/cogs-telemetry":
		return false
	}
	return true
}

func (t *Telemetry) appendAccessLog(line RequestLine) {
	if t.Path == "" {
		return
	}
	if !shouldKeepRecent(line) {
		return
	}
	accessPath := filepath.Join(filepath.Dir(t.Path), "access.log")
	f, err := os.OpenFile(accessPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	cc := line.Country
	if cc != "" {
		cc = "[" + cc + "]"
	}
	internal := ""
	if line.Internal {
		internal = " internal"
	}
	body := ""
	if line.Body != "" {
		body = " | " + line.Body
	}
	_, _ = fmt.Fprintf(f, "%s %s %s %s%s → %d (%dms)%s%s\n",
		line.At.Format(time.RFC3339),
		line.IP,
		line.Method,
		line.Path,
		cc,
		line.Status,
		line.MS,
		body,
		internal,
	)
}

// Middleware wraps HTTP handlers with CoGS request telemetry.
func (t *Telemetry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if t == nil {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		t.total.Add(1)
		cur := t.inFlight.Add(1)
		for {
			peak := t.peakFlight.Load()
			if cur <= peak || t.peakFlight.CompareAndSwap(peak, cur) {
				break
			}
		}
		bodySnippet := captureRequestBody(r)
		rw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		t.inFlight.Add(-1)
		ip := ClientIP(r)
		cc := ClientCountry(r)
		t.noteRequest(RequestLine{
			At:       start.UTC(),
			IP:       ip,
			Country:  cc,
			Internal: IsLikelyInternal(ip),
			Method:   r.Method,
			Path:     r.URL.Path,
			Body:     bodySnippet,
			Status:   rw.status,
			MS:       time.Since(start).Milliseconds(),
		})
	})
}

// RegisterWellKnown exposes JSON snapshot (for local debugging).
func (t *Telemetry) RegisterWellKnown(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/cogs-telemetry", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(t.Snapshot())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// ReadFile loads a persisted snapshot from another app process.
func ReadFile(path string) (Snapshot, error) {
	var s Snapshot
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(b, &s)
	return s, err
}
