package installjob

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const installsDir = ".cogs/installs"

type Store struct {
	wsRoot string
	mu     sync.Mutex
}

func NewStore(wsRoot string) *Store {
	return &Store{wsRoot: wsRoot}
}

func (st *Store) dir() string {
	return filepath.Join(st.wsRoot, installsDir)
}

func (st *Store) jobPath(id string) string {
	return filepath.Join(st.dir(), id+".json")
}

func (st *Store) logPath(id string) string {
	return filepath.Join(st.dir(), id+".log")
}

func (st *Store) Save(job *Job) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if err := os.MkdirAll(st.dir(), 0o750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := st.jobPath(job.ID) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, st.jobPath(job.ID))
}

func (st *Store) Load(id string) (*Job, error) {
	b, err := os.ReadFile(st.jobPath(id))
	if err != nil {
		return nil, err
	}
	var j Job
	if err := json.Unmarshal(b, &j); err != nil {
		return nil, err
	}
	return &j, nil
}

func (st *Store) List() ([]*Job, error) {
	entries, err := os.ReadDir(st.dir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*Job
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".json.tmp") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		j, err := st.Load(id)
		if err != nil {
			continue
		}
		out = append(out, j)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func newJob(id string, req Request) *Job {
	stages := make([]Stage, 0, len(StageOrder))
	for _, sid := range StageOrder {
		st := Stage{ID: sid, Status: StatusPending}
		if sid == StageDone {
			st.Status = StatusPending
		}
		stages = append(stages, st)
	}
	now := nowUTC()
	return &Job{
		ID:        id,
		Status:    StatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
		Request:   req,
		Stages:    stages,
		LogPath:   filepath.Join(installsDir, id+".log"),
	}
}
