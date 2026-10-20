package supervisor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/irgipaulius/cogs/pkg/cockpit"
	"github.com/irgipaulius/cogs/pkg/observe"
	"github.com/irgipaulius/cogs/pkg/workspace"
	"github.com/shirou/gopsutil/v3/process"
)

const metricsHistory = 900

// MetricsSample is one point in time.
type MetricsSample struct {
	At        time.Time            `json:"at"`
	Host      observe.HostSnapshot `json:"host"`
	Apps      []AppMetrics         `json:"apps"`
	Telemetry map[string]cockpit.Snapshot `json:"telemetry,omitempty"`
}

// AppMetrics per workspace app.
type AppMetrics struct {
	Name       string  `json:"name"`
	Running    bool    `json:"running"`
	PID        int     `json:"pid,omitempty"`
	CPUPercent float64 `json:"cpu_percent,omitempty"`
	RSSMB      float64 `json:"rss_mb,omitempty"`
	DBBytes    int64   `json:"db_bytes,omitempty"`
	BlobBytes  int64   `json:"blob_bytes,omitempty"`
}

// MetricsRing stores recent samples.
type MetricsRing struct {
	mu       sync.RWMutex
	samples  []MetricsSample
	wsRoot   string
	ws       workspace.Root
	appPaths map[string]string
}

func NewMetricsRing(wsRoot string, ws workspace.Root) *MetricsRing {
	paths := map[string]string{}
	for _, a := range ws.Apps {
		paths[a.Name] = filepath.Join(wsRoot, a.Path)
	}
	return &MetricsRing{wsRoot: wsRoot, ws: ws, appPaths: paths}
}

func (r *MetricsRing) Start(stop <-chan struct{}) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case t := <-tick.C:
			r.push(r.collect(t))
		}
	}
}

func (r *MetricsRing) push(s MetricsSample) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.samples = append(r.samples, s)
	if len(r.samples) > metricsHistory {
		r.samples = r.samples[len(r.samples)-metricsHistory:]
	}
}

func (r *MetricsRing) Snapshot() []MetricsSample {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]MetricsSample, len(r.samples))
	copy(out, r.samples)
	return out
}

func (r *MetricsRing) Latest() MetricsSample {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.samples) == 0 {
		return r.collect(time.Now())
	}
	return r.samples[len(r.samples)-1]
}

// LatestForStream attaches slim telemetry (no access-log lines) for overview SSE ticks.
func (r *MetricsRing) LatestForStream() MetricsSample {
	s := r.Latest()
	tel := map[string]cockpit.Snapshot{}
	for name, dir := range r.appPaths {
		snap := r.loadCockpit(name, dir)
		snap.Recent = nil
		tel[name] = snap
	}
	s.Telemetry = tel
	return s
}

func (r *MetricsRing) collect(at time.Time) MetricsSample {
	s := MetricsSample{At: at, Host: observe.SnapshotHost()}
	for _, a := range r.ws.Apps {
		dir := r.appPaths[a.Name]
		am := AppMetrics{Name: a.Name}
		pidPath := filepath.Join(dir, ".cogs", "run", "app.pid")
		if b, err := os.ReadFile(pidPath); err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
			if pid > 0 && processAlive(pid) {
				am.Running = true
				am.PID = pid
				if p, err := process.NewProcess(int32(pid)); err == nil {
					if c, err := p.CPUPercent(); err == nil {
						am.CPUPercent = c
					}
					if mi, err := p.MemoryInfo(); err == nil {
						am.RSSMB = float64(mi.RSS) / (1024 * 1024)
					}
				}
			}
		}
		if !am.Running {
			if probeHealth(healthURLForApp(a.Name, dir)) {
				am.Running = true
			}
		}
		reg, _ := LoadRegistry(r.wsRoot)
		for _, db := range reg.Apps {
			if db.AppName == a.Name {
				if st, err := os.Stat(db.Path); err == nil {
					am.DBBytes = st.Size()
				}
				break
			}
		}
		am.BlobBytes = dirSize(filepath.Join(dir, "data", "blobs"))
		s.Apps = append(s.Apps, am)
	}
	return s
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func dirSize(root string) int64 {
	var total int64
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}
