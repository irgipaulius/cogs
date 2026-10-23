package gitrepo

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

// RootOwnedGitFiles returns paths under .git not owned by the current user (if detectable).
func RootOwnedGitFiles(repoRoot string) ([]string, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return nil, nil
	}
	gitDir := filepath.Join(repoRoot, ".git")
	if st, err := os.Stat(gitDir); err != nil || !st.IsDir() {
		return nil, nil
	}
	me, err := user.Current()
	if err != nil || me.Uid == "" {
		return nil, nil
	}
	out, err := exec.Command("find", gitDir, "!", "-uid", me.Uid, "-print").Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// FormatOwnershipWarning explains git pull failures from root-owned objects.
func FormatOwnershipWarning(repoRoot string, paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	n := len(paths)
	sample := paths
	if len(sample) > 3 {
		sample = sample[:3]
	}
	return fmt.Sprintf(
		"git repo %s: %d file(s) under .git are not owned by the CoGS user (e.g. %s) — git pull/fetch may fail; chown -R $(whoami) %s/.git",
		repoRoot, n, strings.Join(sample, ", "), repoRoot,
	)
}
