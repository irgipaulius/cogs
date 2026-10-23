package state

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
)

func repairAppPathsAndConfigs(s *Store, ws wsManifest) error {
	for _, a := range ws.Apps {
		want := strings.TrimSpace(a.Path)
		if want == "" {
			want = "apps/" + a.Name
		}
		want = filepath.ToSlash(want)
		view, err := s.GetApp(a.Name)
		if err == nil {
			cur := filepath.ToSlash(strings.TrimSpace(view.AppPath))
			if cur == "" || strings.Contains(cur, ".cogs/worktrees") || strings.Contains(cur, ".cogs/releases") {
				d := view.AppDesired
				d.AppPath = want
				if d.Present == false {
					d.Present = true
				}
				_ = s.SetDesired(d)
			}
		}
		canon := filepath.Join(s.wsRoot, filepath.FromSlash(want))
		canonCfg := filepath.Join(canon, appcfg.Filename)
		canonB, err := os.ReadFile(canonCfg)
		if err != nil {
			continue
		}
		wtRoot := filepath.Join(s.wsRoot, ".cogs", "worktrees", a.Name)
		entries, _ := os.ReadDir(wtRoot)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			cfgPath := filepath.Join(wtRoot, e.Name(), "apps", a.Name, appcfg.Filename)
			if _, err := os.Stat(cfgPath); err != nil {
				// wrong layout — remove poisoned worktree app config if it points at another app
				_ = filepath.WalkDir(filepath.Join(wtRoot, e.Name()), func(path string, d os.DirEntry, err error) error {
					if err != nil || d.IsDir() || filepath.Base(path) != appcfg.Filename {
						return nil
					}
					if !strings.Contains(path, "/apps/") {
						return nil
					}
					if cfg, lerr := appcfg.Load(filepath.Dir(path)); lerr == nil && cfg.Name != a.Name {
						_ = os.Remove(path)
					}
					return nil
				})
				continue
			}
			if cfg, lerr := appcfg.Load(filepath.Dir(cfgPath)); lerr != nil || strings.TrimSpace(cfg.Name) == "" || cfg.Name != a.Name {
				_ = os.WriteFile(cfgPath, canonB, 0o644)
			}
		}
	}
	return nil
}
