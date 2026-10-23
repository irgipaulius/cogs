package appbuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RestoreTrackedUIArtifacts reverts known tracked UI outputs polluted by older builds.
func RestoreTrackedUIArtifacts(appDir, pagesDir string) error {
	appDir = strings.TrimSpace(appDir)
	if appDir == "" {
		return nil
	}
	if pagesDir == "" {
		pagesDir = "public"
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}
	var relPaths []string
	for _, name := range []string{"config.js", "index.html"} {
		p := filepath.Join(pagesDir, name)
		if _, err := os.Stat(filepath.Join(appDir, p)); err == nil {
			relPaths = append(relPaths, filepath.ToSlash(p))
		}
	}
	if len(relPaths) == 0 {
		return nil
	}
	cmd := exec.Command("git", append([]string{"checkout", "HEAD", "--"}, relPaths...)...)
	cmd.Dir = appDir
	if err := cmd.Run(); err == nil {
		return nil
	}
	cmd = exec.Command("git", append([]string{"restore", "--source=HEAD", "--worktree", "--"}, relPaths...)...)
	cmd.Dir = appDir
	return cmd.Run()
}
