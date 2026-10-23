package gowork

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnsureOverlayHealthy repairs, validates use paths, and runs go work sync.
// Call before any build that relies on .cogs/go.work.
func EnsureOverlayHealthy(wsRoot string) error {
	wsRoot, err := filepath.Abs(wsRoot)
	if err != nil {
		return err
	}
	p := Path(wsRoot)
	if _, err := os.Stat(p); err != nil {
		if err := InitBestEffort(wsRoot); err != nil {
			return err
		}
	}
	if err := RepairOverlay(wsRoot); err != nil {
		return fmt.Errorf("%s: repair: %w", relPath, err)
	}
	uses := readUses(p)
	if len(uses) == 0 {
		return nil
	}
	overlayBase := filepath.Join(wsRoot, overlayDir)
	var bad []string
	for _, u := range uses {
		abs, err := resolveUsePath(overlayBase, u)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s (%v)", u, err))
			continue
		}
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err != nil {
			bad = append(bad, fmt.Sprintf("%s → missing go.mod at %s", u, abs))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%s has broken module paths:\n  %s", relPath, strings.Join(bad, "\n  "))
	}
	cmd := exec.Command("go", "work", "sync")
	cmd.Dir = wsRoot
	cmd.Env = overlayOnlyEnv(p)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go work sync: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func resolveUsePath(overlayDir, use string) (string, error) {
	use = strings.TrimSpace(use)
	if use == "" {
		return "", fmt.Errorf("empty use path")
	}
	if !filepath.IsAbs(use) {
		use = filepath.Join(overlayDir, filepath.FromSlash(use))
	}
	return filepath.Clean(use), nil
}

// IsHermeticBuildDir is true when the module must build without the workspace go.work overlay
// (release worktrees and other detached checkouts).
func IsHermeticBuildDir(wsRoot, buildDir string) bool {
	buildDir = filepath.Clean(buildDir)
	wsRoot = filepath.Clean(wsRoot)
	if strings.Contains(buildDir, string(filepath.Separator)+".cogs"+string(filepath.Separator)+"worktrees"+string(filepath.Separator)) {
		return true
	}
	rel, err := filepath.Rel(wsRoot, buildDir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return true
	}
	return false
}

// BuildEnv returns os.Environ for a Go build in buildDir under wsRoot.
func BuildEnv(wsRoot, buildDir string, base []string) ([]string, error) {
	if base == nil {
		base = os.Environ()
	}
	if IsHermeticBuildDir(wsRoot, buildDir) {
		return stripGOWORK(base), nil
	}
	if err := EnsureOverlayHealthy(wsRoot); err != nil {
		return nil, err
	}
	p := Path(wsRoot)
	if p == "" {
		return base, nil
	}
	if _, err := os.Stat(p); err != nil {
		return base, nil
	}
	return setGOWORK(base, p), nil
}

func overlayOnlyEnv(goworkPath string) []string {
	return setGOWORK(stripGOWORK(os.Environ()), goworkPath)
}

func stripGOWORK(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, "GOWORK=") {
			continue
		}
		out = append(out, e)
	}
	return out
}

func setGOWORK(env []string, path string) []string {
	env = stripGOWORK(env)
	return append(env, "GOWORK="+path)
}

// RemoveModule drops one use path from the overlay (best-effort sync after).
func RemoveModule(wsRoot, moduleRel string) error {
	wsRoot, err := filepath.Abs(wsRoot)
	if err != nil {
		return err
	}
	usePath := UsePathFromRoot(wsRoot, moduleRel)
	p := Path(wsRoot)
	uses := readUses(p)
	var kept []string
	found := false
	for _, u := range uses {
		if u == usePath {
			found = true
			continue
		}
		kept = append(kept, u)
	}
	if !found {
		return nil
	}
	if len(kept) == 0 {
		return os.Remove(p)
	}
	if err := writeUses(p, kept); err != nil {
		return err
	}
	return EnsureOverlayHealthy(wsRoot)
}
