package buildenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnvForBuild returns environment for compiling moduleDir.
// Worktrees and release builds never use the shared overlay go.work.
func EnvForBuild(wsRoot, moduleDir string, base []string) ([]string, error) {
	if base == nil {
		base = os.Environ()
	}
	if IsHermetic(moduleDir) {
		return stripGOWORK(base), nil
	}
	return stripGOWORK(base), nil
}

// IsHermetic is true when the module must not use workspace-root go.work.
func IsHermetic(moduleDir string) bool {
	moduleDir = filepath.Clean(moduleDir)
	return strings.Contains(moduleDir, string(filepath.Separator)+".cogs"+string(filepath.Separator)+"worktrees"+string(filepath.Separator)) ||
		strings.Contains(moduleDir, string(filepath.Separator)+".cogs"+string(filepath.Separator)+"build"+string(filepath.Separator))
}

// PrepareWorktreeBuild creates .cogs/build/<app>/<commit>/go.work for an isolated build.
func PrepareWorktreeBuild(wsRoot, appID, commit, moduleDir string) (goworkPath string, env []string, cleanup func(), err error) {
	wsRoot, err = filepath.Abs(wsRoot)
	if err != nil {
		return "", nil, nil, err
	}
	moduleDir, err = filepath.Abs(moduleDir)
	if err != nil {
		return "", nil, nil, err
	}
	dir := filepath.Join(wsRoot, ".cogs", "build", appID, shortSHA(commit))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", nil, nil, err
	}
	goworkPath = filepath.Join(dir, "go.work")
	relMod, err := filepath.Rel(dir, moduleDir)
	if err != nil {
		return "", nil, nil, err
	}
	relMod = filepath.ToSlash(relMod)
	var b strings.Builder
	b.WriteString("go 1.23.0\n\nuse (\n\t./")
	b.WriteString(relMod)
	b.WriteString("\n)\n")
	if err := os.WriteFile(goworkPath, []byte(b.String()), 0o644); err != nil {
		return "", nil, nil, err
	}
	env = stripGOWORK(os.Environ())
	env = append(env, "GOWORK="+goworkPath)
	cleanup = func() { _ = os.RemoveAll(dir) }
	return goworkPath, env, cleanup, nil
}

func shortSHA(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 12 {
		return s[:12]
	}
	return s
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

// ModuleBuildDir validates module dir exists.
func ModuleBuildDir(moduleDir string) error {
	if st, err := os.Stat(filepath.Join(moduleDir, "go.mod")); err != nil {
		return fmt.Errorf("go.mod missing in %s", moduleDir)
	} else if st.IsDir() {
		return fmt.Errorf("go.mod is directory")
	}
	return nil
}
