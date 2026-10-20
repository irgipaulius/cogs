package observe

import (
	"sync"
	"time"
)

const defaultRingCap = 500

// LogLine is one access or app log entry for the admin stream.
type LogLine struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

// Hub stores recent log lines for SSE and optional file sink.
type Hub struct {
	mu    sync.RWMutex
	lines []LogLine
	cap   int
 subs  map[chan LogLine]struct{}
}

func NewHub(cap int) *Hub {
	if cap <= 0 {
		cap = defaultRingCap
	}
	return &Hub{cap: cap, subs: make(map[chan LogLine]struct{})}
}

func (h *Hub) Append(level, msg string) {
	line := LogLine{Time: time.Now().UTC(), Level: level, Message: msg}
	h.mu.Lock()
	h.lines = append(h.lines, line)
	if len(h.lines) > h.cap {
		h.lines = h.lines[len(h.lines)-h.cap:]
	}
	for ch := range h.subs {
		select {
		case ch <- line:
		default:
		}
	}
	h.mu.Unlock()
}

func (h *Hub) Snapshot() []LogLine {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]LogLine, len(h.lines))
	copy(out, h.lines)
	return out
}

func (h *Hub) Subscribe() chan LogLine {
	ch := make(chan LogLine, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan LogLine) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
	close(ch)
}
