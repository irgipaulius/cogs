// Package hostenv applies the CoGS service user HOME and GOWORK for host CLI tools.
package hostenv

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/gowork"
)

// ServiceHome is HOME for gh/wrangler when CoGS runs as a dedicated user (e.g. /home/cogs).
func ServiceHome() string {
	if h := strings.TrimSpace(os.Getenv("COGS_HOME")); h != "" {
		return h
	}
	return strings.TrimSpace(os.Getenv("HOME"))
}

// CLIEnv returns os.Environ() with service HOME and GOWORK for a workspace.
func CLIEnv(wsRoot string) []string {
	env := os.Environ()
	home := ServiceHome()
	if home != "" {
		env = setEnv(env, "HOME", home)
	}
	if wsRoot != "" {
		if p := gowork.Path(wsRoot); p != "" {
			env = setEnv(env, "GOWORK", p)
		}
	}
	return env
}

func setEnv(env []string, key, val string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	return append(out, prefix+val)
}

// CloneDir returns the standard path for an app's git clone under the workspace.
func CloneDir(wsRoot, repoFull string) string {
	repoFull = strings.TrimSpace(repoFull)
	return filepath.Join(wsRoot, ".cogs", "clones", strings.ReplaceAll(repoFull, "/", "-"))
}
