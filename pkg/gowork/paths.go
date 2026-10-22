package gowork

import (
	"path/filepath"
	"strings"
)

// overlayDir is where the untracked go.work file lives (.cogs).
const overlayDir = ".cogs"

// UsePathFromRoot converts a module path relative to workspace root into a use path for .cogs/go.work.
func UsePathFromRoot(wsRoot, moduleRel string) string {
	wsRoot = filepath.Clean(wsRoot)
	moduleRel = filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(moduleRel), "./"))
	abs := filepath.Join(wsRoot, filepath.FromSlash(moduleRel))
	rel, err := filepath.Rel(filepath.Join(wsRoot, overlayDir), abs)
	if err != nil {
		return "./" + moduleRel
	}
	return "./" + filepath.ToSlash(rel)
}

// NormalizeOverlayUses rewrites use paths copied from repo-root go.work for .cogs/go.work.
func NormalizeOverlayUses(wsRoot string, uses []string) []string {
	out := make([]string, 0, len(uses))
	seen := map[string]struct{}{}
	for _, u := range uses {
		trim := strings.TrimSpace(u)
		if !strings.HasPrefix(trim, "./") && !strings.HasPrefix(trim, "../") {
			continue
		}
		if strings.Contains(trim, "..") || strings.HasPrefix(trim, "./clones/") {
			if _, ok := seen[trim]; !ok {
				seen[trim] = struct{}{}
				out = append(out, trim)
			}
			continue
		}
		mod := strings.TrimPrefix(trim, "./")
		fixed := UsePathFromRoot(wsRoot, mod)
		if _, ok := seen[fixed]; ok {
			continue
		}
		seen[fixed] = struct{}{}
		out = append(out, fixed)
	}
	return out
}

// RepairOverlay rewrites .cogs/go.work use paths when they were stored relative to workspace root.
func RepairOverlay(wsRoot string) error {
	p := Path(wsRoot)
	uses := readUses(p)
	if len(uses) == 0 {
		return nil
	}
	fixed := NormalizeOverlayUses(wsRoot, uses)
	if sameUses(uses, fixed) {
		return nil
	}
	return writeUses(p, fixed)
}

func sameUses(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
