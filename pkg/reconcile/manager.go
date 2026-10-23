package reconcile

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/irgipaulius/cogs/pkg/state"
)

const (
	reconcileBackoffBase = 10 * time.Second
	reconcileBackoffCap  = 15 * time.Minute
)

// Manager runs reconcile jobs asynchronously.
type Manager struct {
	WSRoot string
	DB     *state.Store

	mu       sync.Mutex
	running  map[string]context.CancelFunc
	failures map[string]int
	nextTry  map[string]time.Time
}

// NewManager creates a manager.
func NewManager(st *state.Store) *Manager {
	return &Manager{
		DB:       st,
		WSRoot:   st.WSRoot(),
		running:  map[string]context.CancelFunc{},
		failures: map[string]int{},
		nextTry:  map[string]time.Time{},
	}
}

// Enqueue starts reconcile for appID (deduped per app, with exponential backoff after failures).
func (m *Manager) Enqueue(appID string, opts Options) {
	m.mu.Lock()
	if t, ok := m.nextTry[appID]; ok && time.Now().Before(t) {
		m.mu.Unlock()
		return
	}
	if _, ok := m.running[appID]; ok {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.running[appID] = cancel
	m.mu.Unlock()
	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.running, appID)
			m.mu.Unlock()
			cancel()
		}()
		if err := Run(ctx, m.DB, appID, opts); err != nil {
			log.Printf("reconcile %s: %v", appID, err)
			m.scheduleRetry(appID)
			return
		}
		m.clearRetry(appID)
	}()
}

func (m *Manager) scheduleRetry(appID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures[appID]++
	n := m.failures[appID]
	delay := reconcileBackoffBase
	for i := 1; i < n; i++ {
		delay *= 2
		if delay >= reconcileBackoffCap {
			delay = reconcileBackoffCap
			break
		}
	}
	m.nextTry[appID] = time.Now().Add(delay)
}

func (m *Manager) clearRetry(appID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failures, appID)
	delete(m.nextTry, appID)
}
