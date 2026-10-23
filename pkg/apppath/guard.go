package apppath

import (
	"path/filepath"
	"strings"
)

// IsEphemeral returns true for worktrees, releases, clones (never patch or register).
func IsEphemeral(appDir string) bool {
	p := filepath.ToSlash(appDir)
	return strings.Contains(p, "/.cogs/worktrees/") ||
		strings.Contains(p, "/.cogs/releases/") ||
		strings.Contains(p, "/.cogs/clones/") ||
		strings.Contains(p, "/.cogs/build/")
}
