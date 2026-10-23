package gitrepo

import (
	"context"
	"os/exec"
	"strings"
)

// IsAncestor reports whether ancestor is an ancestor of commit in gitRoot (same repo).
// ShouldSkipDeployCommit reports whether candidate is already covered by current release.
func ShouldSkipDeployCommit(ctx context.Context, gitRoot, candidate, current string) bool {
	candidate = strings.TrimSpace(candidate)
	current = strings.TrimSpace(current)
	if candidate == "" {
		return true
	}
	if current == "" {
		return false
	}
	if strings.EqualFold(candidate, current) {
		return true
	}
	// current is newer on main: skip strictly older commits.
	if IsAncestor(ctx, gitRoot, candidate, current) {
		return true
	}
	return false
}

// IsAncestor reports whether ancestor is an ancestor of commit in gitRoot (same repo).
func IsAncestor(ctx context.Context, gitRoot, ancestor, commit string) bool {
	ancestor = strings.TrimSpace(ancestor)
	commit = strings.TrimSpace(commit)
	if ancestor == "" || commit == "" {
		return false
	}
	if strings.EqualFold(ancestor, commit) {
		return true
	}
	cmd := exec.CommandContext(ctx, "git", "-C", gitRoot, "merge-base", "--is-ancestor", ancestor, commit)
	return cmd.Run() == nil
}
