package reconcile

import (
	"context"
	"log"
	"sync"

	"github.com/irgipaulius/cogs/pkg/state"
)

// Manager runs reconcile jobs asynchronously.
type Manager struct {
	WSRoot string
	DB     *state.Store

	mu      sync.Mutex
	running map[string]context.CancelFunc
}

// NewManager creates a manager.
func NewManager(st *state.Store) *Manager {
	return &Manager{DB: st, WSRoot: st.WSRoot(), running: map[string]context.CancelFunc{}}
}

// Enqueue starts reconcile for appID (deduped per app).
func (m *Manager) Enqueue(appID string, opts Options) {
	m.mu.Lock()
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
		}
	}()
}
