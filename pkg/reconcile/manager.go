package reconcile

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/irgipaulius/cogs/pkg/gitrepo"
	"github.com/irgipaulius/cogs/pkg/state"
	"github.com/irgipaulius/cogs/pkg/wranglerexec"
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
	pending  map[string]Options
	failures map[string]int
	nextTry  map[string]time.Time
}

// NewManager creates a manager.
func NewManager(st *state.Store) *Manager {
	return &Manager{
		DB:       st,
		WSRoot:   st.WSRoot(),
		running:  map[string]context.CancelFunc{},
		pending:  map[string]Options{},
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
		m.mergePending(appID, opts)
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.running[appID] = cancel
	m.mu.Unlock()
	go m.runLoop(appID, ctx, opts, cancel)
}

func (m *Manager) runLoop(appID string, ctx context.Context, opts Options, cancel context.CancelFunc) {
	defer func() {
		m.mu.Lock()
		delete(m.running, appID)
		m.mu.Unlock()
		cancel()
	}()
	for {
		if err := Run(ctx, m.DB, appID, opts); err != nil {
			if strings.Contains(err.Error(), "reconcile lock timeout") {
				m.mu.Lock()
				m.mergePending(appID, opts)
				m.mu.Unlock()
			} else {
				log.Printf("reconcile %s: %v", appID, err)
				m.scheduleRetry(appID)
			}
		} else {
			m.clearRetry(appID)
		}
		if m.WSRoot != "" {
			wranglerexec.RotateWorkspaceLog(filepath.Join(m.WSRoot, ".cogs", "run", "wrangler.log"))
		}
		m.mu.Lock()
		next, ok := m.pending[appID]
		if ok {
			delete(m.pending, appID)
		}
		m.mu.Unlock()
		if !ok {
			return
		}
		opts = next
	}
}

func (m *Manager) mergePending(appID string, opts Options) {
	cur, ok := m.pending[appID]
	if !ok {
		m.pending[appID] = opts
		return
	}
	if opts.Commit != "" {
		if cur.Commit == "" || m.commitWins(opts.Commit, cur.Commit) {
			cur.Commit = opts.Commit
		}
	}
	if opts.ForceRebuild {
		cur.ForceRebuild = true
	}
	if len(opts.StagesOnly) > 0 {
		cur.StagesOnly = opts.StagesOnly
	}
	m.pending[appID] = cur
}

func (m *Manager) commitWins(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return false
	}
	if b == "" {
		return true
	}
	if m.WSRoot != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if gitrepo.IsAncestor(ctx, m.WSRoot, a, b) {
			return false
		}
		if gitrepo.IsAncestor(ctx, m.WSRoot, b, a) {
			return true
		}
	}
	return a > b
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
