package gomod

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CheckLocalReplace fails when go.mod contains a replace => local path that does not exist on this host.
func CheckLocalReplace(appDir string) error {
	path := filepath.Join(appDir, "go.mod")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "replace ") {
			continue
		}
		_, rhs, ok := strings.Cut(line, "=>")
		if !ok {
			continue
		}
		rhs = strings.TrimSpace(strings.TrimSuffix(rhs, "//"))
		rhs = strings.Trim(rhs, `"`)
		if rhs == "" || strings.Contains(rhs, "://") {
			continue
		}
		// Version-only replace (=> v1.2.3) is fine.
		if strings.HasPrefix(rhs, "v") && !strings.Contains(rhs, "/") && !strings.Contains(rhs, `\`) {
			continue
		}
		abs := rhs
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(appDir, abs)
		}
		if st, err := os.Stat(abs); err != nil || !st.IsDir() {
			return fmt.Errorf(
				"go.mod replace points at %q which is missing on this server — remove the replace or vendor cogs; dev-only replaces must not be committed",
				rhs,
			)
		}
	}
	return nil
}
