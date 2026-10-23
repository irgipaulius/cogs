package installjob

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/irgipaulius/cogs/pkg/gitpublic"
	"github.com/irgipaulius/cogs/pkg/provision"
)

// Manager runs install jobs asynchronously with persistence.
type Manager struct {
	WSRoot string
	Store  *Store

	mu          sync.RWMutex
	subscribers map[chan struct{}]struct{}
	running     map[string]context.CancelFunc
}

func NewManager(wsRoot string) *Manager {
	return &Manager{
		WSRoot:      wsRoot,
		Store:       NewStore(wsRoot),
		subscribers: map[chan struct{}]struct{}{},
		running:     map[string]context.CancelFunc{},
	}
}

// ResumePending restarts jobs that were running when the process exited.
func (m *Manager) ResumePending() {
	jobs, err := m.Store.List()
	if err != nil {
		return
	}
	for _, j := range jobs {
		if j.Status != StatusRunning {
			continue
		}
		fresh := newJob(j.ID, j.Request)
		fresh.CreatedAt = j.CreatedAt
		_ = m.Store.Save(fresh)
		m.startJob(fresh, false)
	}
}

func (m *Manager) Subscribe() chan struct{} {
	ch := make(chan struct{}, 8)
	m.mu.Lock()
	m.subscribers[ch] = struct{}{}
	m.mu.Unlock()
	return ch
}

func (m *Manager) Unsubscribe(ch chan struct{}) {
	m.mu.Lock()
	delete(m.subscribers, ch)
	m.mu.Unlock()
}

func (m *Manager) notify() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for ch := range m.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Create enqueues a new install and returns the job id.
func (m *Manager) Create(ctx context.Context, req Request) (*Job, error) {
	req.RepoFull = gitpublic.NormalizeFullName(req.RepoFull)
	if req.RepoFull == "" && !req.ScaffoldOnly {
		return nil, fmt.Errorf("repo required (owner/name)")
	}
	if req.FinishApp == "" && req.RepoFull != "" {
		if existing := provision.InstalledByRepo(m.WSRoot, req.RepoFull); existing != "" {
			return nil, fmt.Errorf("repository already installed as %q — remove it first or use Finish install on that app", existing)
		}
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	job := newJob(id, req)
	if job.AppName == "" {
		job.AppName = strings.TrimSpace(req.AppName)
	}
	if job.AppName == "" {
		job.AppName = strings.TrimSpace(req.FinishApp)
	}
	if job.AppName == "" && req.RepoFull != "" {
		if i := strings.LastIndex(req.RepoFull, "/"); i >= 0 && i+1 < len(req.RepoFull) {
			job.AppName = req.RepoFull[i+1:]
		}
	}
	if err := m.Store.Save(job); err != nil {
		return nil, err
	}
	m.notify()
	m.startJob(job, false)
	return job, nil
}

// Retry reruns a failed job from the beginning (clone is reused when present).
func (m *Manager) Retry(ctx context.Context, id string) (*Job, error) {
	job, err := m.Store.Load(id)
	if err != nil {
		return nil, err
	}
	if job.Status != StatusFailed {
		return nil, fmt.Errorf("job is not failed")
	}
	fresh := newJob(id, job.Request)
	fresh.CreatedAt = job.CreatedAt
	if err := m.Store.Save(fresh); err != nil {
		return nil, err
	}
	m.notify()
	m.startJob(fresh, false)
	return fresh, nil
}

func (m *Manager) Get(id string) (*Job, error) {
	return m.Store.Load(id)
}

func (m *Manager) List() ([]*Job, error) {
	return m.Store.List()
}

func (m *Manager) LogTail(id string, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = 256 * 1024
	}
	b, err := readTail(m.Store.logPath(id), maxBytes)
	return string(b), err
}

func (m *Manager) startJob(job *Job, resume bool) {
	m.mu.Lock()
	if _, ok := m.running[job.ID]; ok {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.running[job.ID] = cancel
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			delete(m.running, job.ID)
			m.mu.Unlock()
			cancel()
		}()
		runner := &Runner{
			WSRoot: m.WSRoot,
			Store:  m.Store,
			onSave: func(*Job) { m.notify() },
		}
		if resume {
			// Reset running marker and continue from current stage index.
			runner.Run(ctx, job)
			return
		}
		runner.Run(ctx, job)
	}()
}

func newID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func readTail(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	if size <= max {
		return os.ReadFile(path)
	}
	buf := make([]byte, max)
	if _, err := f.ReadAt(buf, size-max); err != nil {
		return nil, err
	}
	return buf, nil
}
