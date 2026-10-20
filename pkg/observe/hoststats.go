package observe

import (
	"os"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// HostSnapshot is CPU/RAM for the admin dashboard.
type HostSnapshot struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemUsedMB  float64 `json:"mem_used_mb"`
	MemTotalMB float64 `json:"mem_total_mb"`
	RSSMB      float64 `json:"process_rss_mb"`
}

// SnapshotHost returns current host CPU/RAM for dashboards.
func SnapshotHost() HostSnapshot {
	return hostSnapshot()
}

func hostSnapshot() HostSnapshot {
	var out HostSnapshot
	if p, err := cpu.Percent(0, false); err == nil && len(p) > 0 {
		out.CPUPercent = p[0]
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		out.MemUsedMB = float64(vm.Used) / (1024 * 1024)
		out.MemTotalMB = float64(vm.Total) / (1024 * 1024)
	}
	if proc, err := process.NewProcess(int32(os.Getpid())); err == nil {
		if mi, err := proc.MemoryInfo(); err == nil {
			out.RSSMB = float64(mi.RSS) / (1024 * 1024)
		}
	}
	return out
}
