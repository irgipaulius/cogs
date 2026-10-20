package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// Collector exposes simple Prometheus text metrics.
type Collector struct {
	requests atomic.Uint64
}

// IncRequest increments the request counter.
func (c *Collector) IncRequest() { c.requests.Add(1) }

// Handler serves GET /metrics in Prometheus text format.
func (c *Collector) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP cogs_http_requests_total Total HTTP requests\n")
		fmt.Fprintf(w, "# TYPE cogs_http_requests_total counter\n")
		fmt.Fprintf(w, "cogs_http_requests_total %d\n", c.requests.Load())
	})
}
