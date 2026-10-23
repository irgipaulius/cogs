package gitrepo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ServiceUser is the CoGS service account (not the user running doctor).
func ServiceUser() string {
	if u := strings.TrimSpace(os.Getenv("COGS_SERVICE_USER")); u != "" {
		return u
	}
	return "cogs"
}

// RootOwnedGitFiles returns paths under .git not owned by the CoGS service user.
func RootOwnedGitFiles(repoRoot string) ([]string, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return nil, nil
	}
	gitDir := filepath.Join(repoRoot, ".git")
	if st, err := os.Stat(gitDir); err != nil || !st.IsDir() {
		return nil, nil
	}
	uid, err := serviceUserUID()
	if err != nil || uid == "" {
		return nil, nil
	}
	out, err := exec.Command("find", gitDir, "!", "-uid", uid, "-print").Output()
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

func serviceUserUID() (string, error) {
	user := ServiceUser()
	out, err := exec.Command("id", "-u", user).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// FormatOwnershipWarning explains git pull failures from wrong .git ownership.
func FormatOwnershipWarning(repoRoot string, paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	n := len(paths)
	sample := paths
	if len(sample) > 3 {
		sample = sample[:3]
	}
	user := ServiceUser()
	return fmt.Sprintf(
		"git repo %s: %d file(s) under .git are not owned by user %q (e.g. %s) — git pull/fetch may fail; chown -R %s:%s %s/.git",
		repoRoot, n, user, strings.Join(sample, ", "), user, user, repoRoot,
	)
}
