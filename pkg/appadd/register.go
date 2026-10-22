package appadd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

// RegisterFromDir adds an existing app directory (with cogs.app.toml) to the workspace.
func RegisterFromDir(wsRoot, name, appDir, gitRepo, deployMode string) (string, error) {
	name = strings.TrimSpace(name)
	appDir, err := filepath.Abs(appDir)
	if err != nil {
		return "", err
	}
	wsRoot, err = filepath.Abs(wsRoot)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(appDir, appcfg.Filename)); err != nil {
		return "", fmt.Errorf("missing %s in %s", appcfg.Filename, appDir)
	}
	app, err := appcfg.Load(appDir)
	if err != nil {
		return "", err
	}
	if name == "" {
		name = app.Name
	}
	if name == "" {
		return "", fmt.Errorf("app name required")
	}
	rel, err := filepath.Rel(wsRoot, appDir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("app dir must be inside workspace root")
	}
	rel = filepath.ToSlash(rel)

	_, ws, err := workspace.FindRoot(wsRoot)
	if err != nil {
		return "", err
	}
	for _, a := range ws.Apps {
		if a.Name == name {
			return "", fmt.Errorf("app %q already exists", name)
		}
	}
	if err := patchGitMeta(appDir, gitRepo, deployMode); err != nil {
		return "", err
	}
	ws.Apps = append(ws.Apps, workspace.App{
		Name:        name,
		Path:        rel,
		Description: "Installed from GitHub",
	})
	b, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	if err := os.WriteFile(filepath.Join(wsRoot, workspace.FileName), b, 0o644); err != nil {
		return "", err
	}
	if hasGoMod(appDir) {
		cmd := exec.Command("go", "work", "use", "./"+rel)
		cmd.Dir = wsRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("go work use: %w: %s", err, string(out))
		}
	}
	return fmt.Sprintf("registered %s at %s", name, rel), nil
}

func hasGoMod(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

func patchGitMeta(appDir, gitRepo, deployMode string) error {
	path := filepath.Join(appDir, appcfg.Filename)
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	if gitRepo = strings.TrimSpace(gitRepo); gitRepo != "" {
		lines = setKey(lines, "git_repo", fmt.Sprintf("%q", gitRepo))
	}
	if deployMode = strings.TrimSpace(deployMode); deployMode != "" {
		lines = setKey(lines, "deploy_mode", fmt.Sprintf("%q", deployMode))
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

func setKey(lines []string, key, val string) []string {
	prefix := key + " ="
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, prefix) || strings.HasPrefix(trim, key+"=") {
			lines[i] = key + " = " + val
			return lines
		}
	}
	return append(lines, key+" = "+val)
}
