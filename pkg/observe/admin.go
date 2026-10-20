package observe

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// RegisterAdminRoutes mounts dashboard UI + JSON/SSE APIs (loopback Host only).
func (r *Runtime) RegisterAdminRoutes(mux *http.ServeMux, dbPath string, db *sql.DB) {
	if r == nil || !r.DashboardEnabled() {
		return
	}
	h := LocalAdminOnly(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/admin", "/admin/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(adminHTML))
		case "/admin/api/snapshot":
			writeJSON(w, map[string]any{
				"host": hostSnapshot(),
				"http": map[string]any{
					"total":     r.HTTP.Total.Load(),
					"in_flight": r.HTTP.InFlight.Load(),
					"last_ms":   r.HTTP.LastMS.Load(),
				},
				"db": dbSnapshot(dbPath, db, r.SQL),
			})
		case "/admin/api/logs":
			writeJSON(w, r.Hub.Snapshot())
		case "/admin/api/stream":
			streamLogs(w, req, r.Hub)
		default:
			http.NotFound(w, req)
		}
	}))
	mux.Handle("/admin", h)
	mux.Handle("/admin/", h)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func streamLogs(w http.ResponseWriter, req *http.Request, hub *Hub) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)
	for _, line := range hub.Snapshot() {
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(line))
		fl.Flush()
	}
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-req.Context().Done():
			return
		case line := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", mustJSON(line))
			fl.Flush()
		case <-tick.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			fl.Flush()
		}
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
