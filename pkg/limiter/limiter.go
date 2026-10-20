// Package limiter provides IP- and token-keyed rate limiting with an LRU cache.
package limiter

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Clock returns the current time; inject a fake clock in tests.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// LRU wraps rate.Limiter instances keyed by string, evicting oldest when full.
type LRU struct {
	mu      sync.Mutex
	entries map[string]*entry
	order   []string
	max     int
	r       rate.Limit
	b       int
	clock   Clock
}

type entry struct {
	lim *rate.Limiter
	at  time.Time
}

// NewLRU creates a limiter cache. r is events per second; b is burst.
func NewLRU(maxKeys int, r rate.Limit, burst int) *LRU {
	return &LRU{
		entries: make(map[string]*entry),
		max:     maxKeys,
		r:       r,
		b:       burst,
		clock:   realClock{},
	}
}

// SetClock replaces the clock (tests only).
func (l *LRU) SetClock(c Clock) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.clock = c
}

func (l *LRU) get(key string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.clock == nil {
		l.clock = realClock{}
	}
	now := l.clock.Now()
	if e, ok := l.entries[key]; ok {
		e.at = now
		l.touch(key)
		return e.lim
	}
	lim := rate.NewLimiter(l.r, l.b)
	l.entries[key] = &entry{lim: lim, at: now}
	l.order = append(l.order, key)
	for len(l.order) > l.max {
		evict := l.order[0]
		l.order = l.order[1:]
		delete(l.entries, evict)
	}
	return lim
}

func (l *LRU) touch(key string) {
	for i, k := range l.order {
		if k == key {
			l.order = append(l.order[:i], l.order[i+1:]...)
			l.order = append(l.order, key)
			return
		}
	}
}

// Allow reports whether one event is allowed for key.
func (l *LRU) Allow(key string) bool {
	return l.get(key).Allow()
}

// Middleware limits by header value (e.g. CF-Connecting-IP).
func Middleware(l *LRU, header string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get(header)
			if key == "" {
				key = r.RemoteAddr
			}
			if !l.Allow(key) {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
