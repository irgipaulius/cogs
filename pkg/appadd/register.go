package appadd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/apppath"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

// RegisterFromDir adds an existing app directory (with cogs.app.toml) to the untracked installed overlay.
func RegisterFromDir(wsRoot, name, appDir, gitRepo, deployMode, gitRootRel string) (string, error) {
	name = strings.TrimSpace(name)
	appDir, err := filepath.Abs(appDir)
	if err != nil {
		return "", err
	}
	wsRoot, err = filepath.Abs(wsRoot)
	if err != nil {
		return "", err
	}
	if apppath.IsEphemeral(appDir) {
		return "", fmt.Errorf("refusing to register ephemeral app dir %s", appDir)
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
	if err := patchGitMeta(appDir, gitRepo, deployMode, "", "", false); err != nil {
		return "", err
	}
	gitRootRel = filepath.ToSlash(strings.TrimSpace(gitRootRel))
	if err := workspace.SaveInstalledApp(wsRoot, workspace.InstalledApp{
		App: workspace.App{
			Name:        name,
			Path:        rel,
			Description: "Installed from GitHub",
		},
		GitRepo: strings.TrimSpace(gitRepo),
		GitRoot: gitRootRel,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("registered %s at %s", name, rel), nil
}

func hasGoMod(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err != nil
}

// PatchAppMeta updates cogs.app.toml before register (install overrides).
// When pageApex is true, page_subdomain is cleared so the UI serves on the zone apex.
func PatchAppMeta(appDir, gitRepo, deployMode, zone, pageSub string, pageApex bool) error {
	if apppath.IsEphemeral(appDir) {
		return fmt.Errorf("refusing to patch %s under build artifact path", appcfg.Filename)
	}
	if pageApex {
		pageSub = ""
	}
	return patchGitMeta(appDir, gitRepo, deployMode, zone, pageSub, pageApex)
}

func patchGitMeta(appDir, gitRepo, deployMode, zone, pageSub string, pageApex bool) error {
	path := filepath.Join(appDir, appcfg.Filename)
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	orig := string(b)
	lines := strings.Split(orig, "\n")
	if gitRepo = strings.TrimSpace(gitRepo); gitRepo != "" {
		lines = setKey(lines, "git_repo", fmt.Sprintf("%q", gitRepo))
	}
	if deployMode = strings.TrimSpace(deployMode); deployMode != "" {
		lines = setKey(lines, "deploy_mode", fmt.Sprintf("%q", deployMode))
	}
	if zone = strings.TrimSpace(zone); zone != "" {
		lines = setKey(lines, "zone", fmt.Sprintf("%q", zone))
	}
	if pageApex {
		lines = setKey(lines, "page_subdomain", `""`)
	} else if pageSub = strings.TrimSpace(pageSub); pageSub != "" {
		lines = setKey(lines, "page_subdomain", fmt.Sprintf("%q", pageSub))
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	if out == orig || out == orig+"\n" {
		return nil
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
