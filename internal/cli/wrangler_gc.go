package cli

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/irgipaulius/cogs/pkg/hostenv"
	"github.com/irgipaulius/cogs/pkg/wranglerpaths"
)

const wranglerRunLogMax = 2 << 20 // 2 MiB

const wranglerLogKeep = 200

func gcWranglerLogs(wsRoot string) {
	if wsRoot != "" {
		rotateWranglerLog(filepath.Join(wsRoot, ".cogs", "run", "wrangler.log"))
	}
	home := hostenv.ServiceHome()
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		return
	}
	dirs := []string{
		filepath.Join(home, ".wrangler", "logs"),
		filepath.Join(home, ".config", ".wrangler", "logs"),
	}
	if leg := wranglerpaths.LegacyHomeDir(); leg != "" {
		dirs = append(dirs, filepath.Join(leg, "logs"))
	}
	var removed int
	for _, dir := range dirs {
		removed += gcLogDir(dir)
	}
	if removed > 0 {
		log.Printf("gc: removed %d old wrangler log files", removed)
	}
}

func gcLogDir(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	type fi struct {
		path string
		mod  time.Time
	}
	var files []fi
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fi{filepath.Join(dir, e.Name()), info.ModTime()})
	}
	if len(files) <= wranglerLogKeep {
		return 0
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	removed := 0
	cutoff := time.Now().Add(-72 * time.Hour)
	for i := wranglerLogKeep; i < len(files); i++ {
		if files[i].mod.Before(cutoff) || i >= wranglerLogKeep*2 {
			if os.Remove(files[i].path) == nil {
				removed++
			}
		}
	}
	return removed
}

func rotateWranglerLog(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= wranglerRunLogMax {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	keep := b
	if len(keep) > wranglerRunLogMax/2 {
		keep = keep[len(keep)-wranglerRunLogMax/2:]
	}
	_ = os.WriteFile(path, append([]byte("… truncated …\n"), keep...), 0o644)
}
