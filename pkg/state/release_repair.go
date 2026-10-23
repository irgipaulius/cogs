package state

import (
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/release"
)

func repairReleaseState(s *Store, ws wsManifest) error {
	for _, a := range ws.Apps {
		if strings.TrimSpace(a.Name) == "" {
			continue
		}
		appDir := filepath.Join(s.wsRoot, filepath.FromSlash(a.Path))
		combined := a.Name == "admin"
		if cfg, err := appcfg.Load(appDir); err == nil {
			combined = cfg.Kind == "combined"
		}
		rels, err := s.ListReleaseRows(a.Name)
		if err != nil {
			return err
		}
		for _, r := range rels {
			if _, _, ok := release.BinaryForCommit(s.wsRoot, a.Name, r.CommitSHA, combined); ok {
				continue
			}
			_ = s.DeleteReleaseRow(a.Name, r.ID)
		}
		commit, bin, ok := release.BestRelease(s.wsRoot, a.Name, combined)
		if !ok {
			continue
		}
		id, err := s.UpsertRelease(a.Name, commit, bin, true)
		if err != nil {
			return err
		}
		if id > 0 {
			_ = s.ActivateRelease(a.Name, id)
		}
		_, _ = release.SyncPointer(s.wsRoot, a.Name, commit, combined)
	}
	return nil
}
