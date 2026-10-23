package reconcile

import (
	"testing"
	"time"
)

func TestManagerBackoffBlocksImmediateRetry(t *testing.T) {
	m := &Manager{
		failures: map[string]int{},
		nextTry:  map[string]time.Time{},
	}
	m.scheduleRetry("todo")
	m.mu.Lock()
	until := m.nextTry["todo"]
	m.mu.Unlock()
	if !time.Now().Before(until) {
		t.Fatalf("expected future retry, got %v", until)
	}
	if until.Sub(time.Now()) < reconcileBackoffBase/2 {
		t.Fatalf("backoff too short: %v", until.Sub(time.Now()))
	}
}
