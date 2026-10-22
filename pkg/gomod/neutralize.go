package gomod

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FixLocalReplace removes or fixes replace directives pointing at missing local paths.
// When wsRoot contains the CoGS module, a replace targeting github.com/irgipaulius/cogs is dropped
// so GOWORK can supply the module.
func FixLocalReplace(appDir, wsRoot string) (changed bool, err error) {
	path := filepath.Join(appDir, "go.mod")
	b, err := os.ReadFile(path)
	if err != nil {
		return false, nil
	}
	lines := strings.Split(string(b), "\n")
	var out []string
	var removed []string
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "replace ") {
			out = append(out, line)
			continue
		}
		_, rhs, ok := strings.Cut(trim, "=>")
		if !ok {
			out = append(out, line)
			continue
		}
		rhs = strings.TrimSpace(strings.TrimSuffix(rhs, "//"))
		rhs = strings.Trim(rhs, `"`)
		if rhs == "" || strings.Contains(rhs, "://") {
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(rhs, "v") && !strings.Contains(rhs, "/") && !strings.Contains(rhs, `\`) {
			out = append(out, line)
			continue
		}
		abs := rhs
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(appDir, abs)
		}
		if st, statErr := os.Stat(abs); statErr == nil && st.IsDir() {
			out = append(out, line)
			continue
		}
		// Missing local path — drop replace if it targets the cogs module path on the left.
		lhs := strings.TrimPrefix(trim, "replace ")
		lhs, _, _ = strings.Cut(lhs, "=>")
		lhs = strings.TrimSpace(strings.Trim(lhs, `"`))
		if strings.Contains(lhs, "github.com/irgipaulius/cogs") && wsRoot != "" {
			removed = append(removed, trim)
			changed = true
			continue
		}
		return false, fmt.Errorf(
			"go.mod has replace %q pointing at missing path %q — remove that line on the server or commit a portable module layout",
			trim,
			rhs,
		)
	}
	if !changed {
		return false, CheckLocalReplace(appDir)
	}
	body := strings.Join(out, "\n")
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return false, err
	}
	if len(removed) > 0 {
		return true, nil
	}
	return changed, CheckLocalReplace(appDir)
}
