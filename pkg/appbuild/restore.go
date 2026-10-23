package appbuild

import (
	"bytes"
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
	if len(relPaths) > 0 {
		cmd := exec.Command("git", append([]string{"checkout", "HEAD", "--"}, relPaths...)...)
		cmd.Dir = appDir
		if err := cmd.Run(); err != nil {
			cmd = exec.Command("git", append([]string{"restore", "--source=HEAD", "--worktree", "--"}, relPaths...)...)
			cmd.Dir = appDir
			if err := cmd.Run(); err != nil {
				return err
			}
		}
	}
	return RemoveUntrackedPublicBuildOutput(appDir, pagesDir)
}

// RemoveUntrackedPublicBuildOutput deletes all untracked files under public/ (build droppings).
func RemoveUntrackedPublicBuildOutput(appDir, pagesDir string) error {
	if pagesDir == "" {
		pagesDir = "public"
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all", "--", pagesDir)
	cmd.Dir = appDir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	for _, line := range bytes.Split(out, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) < 4 || line[0] != '?' || line[1] != '?' {
			continue
		}
		rel := strings.TrimSpace(string(line[3:]))
		if rel == "" {
			continue
		}
		_ = os.RemoveAll(filepath.Join(appDir, filepath.FromSlash(rel)))
	}
	return RemoveUntrackedUIHashAssets(appDir, pagesDir)
}

// RemoveUntrackedUIHashAssets deletes untracked Vite hashed bundles under public/assets/.
func RemoveUntrackedUIHashAssets(appDir, pagesDir string) error {
	if pagesDir == "" {
		pagesDir = "public"
	}
	assets := filepath.Join(appDir, pagesDir, "assets")
	entries, err := os.ReadDir(assets)
	if err != nil {
		return nil
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || !strings.HasPrefix(name, "index-") {
			continue
		}
		if !strings.HasSuffix(name, ".js") && !strings.HasSuffix(name, ".css") {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(pagesDir, "assets", name))
		cmd := exec.Command("git", "ls-files", "--error-unmatch", rel)
		cmd.Dir = appDir
		if cmd.Run() == nil {
			continue
		}
		_ = os.Remove(filepath.Join(assets, name))
	}
	return nil
}
