package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const installedFile = ".cogs/installed.json"

// InstalledApp is an admin-installed app (untracked overlay).
type InstalledApp struct {
	App
	GitRepo string `json:"git_repo,omitempty"`
	GitRoot string `json:"git_root,omitempty"` // relative to workspace root, e.g. .cogs/clones/owner-repo
}

type installedManifest struct {
	Apps []InstalledApp `json:"apps"`
}

func installedPath(wsRoot string) string {
	return filepath.Join(wsRoot, installedFile)
}

func loadInstalled(wsRoot string) installedManifest {
	var m installedManifest
	b, err := os.ReadFile(installedPath(wsRoot))
	if err != nil {
		return m
	}
	_ = json.Unmarshal(b, &m)
	return m
}

// MergeInstalled overlays .cogs/installed.json onto the shipped workspace manifest.
func MergeInstalled(wsRoot string, base Root) Root {
	inst := loadInstalled(wsRoot)
	if len(inst.Apps) == 0 {
		return base
	}
	byName := map[string]int{}
	for i, a := range base.Apps {
		byName[a.Name] = i
	}
	for _, ia := range inst.Apps {
		if i, ok := byName[ia.Name]; ok {
			base.Apps[i] = ia.App
		} else {
			base.Apps = append(base.Apps, ia.App)
			byName[ia.Name] = len(base.Apps) - 1
		}
	}
	return base
}

// RemoveInstalledApp drops one app from the installed overlay (no-op if missing).
func RemoveInstalledApp(wsRoot, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return os.ErrInvalid
	}
	m := loadInstalled(wsRoot)
	out := m.Apps[:0]
	found := false
	for _, a := range m.Apps {
		if a.Name == name {
			found = true
			continue
		}
		out = append(out, a)
	}
	if !found {
		return nil
	}
	m.Apps = out
	if len(m.Apps) == 0 {
		return os.Remove(installedPath(wsRoot))
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(installedPath(wsRoot), b, 0o644)
}

// SaveInstalledApp appends or replaces one installed app record.
func SaveInstalledApp(wsRoot string, rec InstalledApp) error {
	rec.Name = strings.TrimSpace(rec.Name)
	if rec.Name == "" {
		return os.ErrInvalid
	}
	m := loadInstalled(wsRoot)
	found := false
	for i, a := range m.Apps {
		if a.Name == rec.Name {
			m.Apps[i] = rec
			found = true
			break
		}
	}
	if !found {
		m.Apps = append(m.Apps, rec)
	}
	if err := os.MkdirAll(filepath.Join(wsRoot, ".cogs"), 0o750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(installedPath(wsRoot), b, 0o644)
}

// InstalledLookup returns overlay metadata for an app name.
func InstalledLookup(wsRoot, name string) (InstalledApp, bool) {
	for _, a := range loadInstalled(wsRoot).Apps {
		if a.Name == name {
			return a, true
		}
	}
	return InstalledApp{}, false
}

// GitRootForApp resolves the git repository root used for fetch/worktrees.
func GitRootForApp(wsRoot string, appName string, gitRepo string) string {
	if ia, ok := InstalledLookup(wsRoot, appName); ok && strings.TrimSpace(ia.GitRoot) != "" {
		return filepath.Join(wsRoot, ia.GitRoot)
	}
	gitRepo = strings.TrimSpace(gitRepo)
	if gitRepo != "" {
		clone := filepath.Join(wsRoot, ".cogs", "clones", strings.ReplaceAll(gitRepo, "/", "-"))
		if isGitDir(clone) {
			return clone
		}
	}
	return wsRoot
}

func isGitDir(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
