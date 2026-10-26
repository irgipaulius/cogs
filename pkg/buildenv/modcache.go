package buildenv

import (
	"os"
	"path/filepath"
	"strings"
)

// EnsureModCacheOutsideWorkspace redirects GOMODCACHE when it lives inside wsRoot.
// A module cache under the workspace breaks `go test ./...` with go.work ("not one of the workspace modules").
func EnsureModCacheOutsideWorkspace(wsRoot string, env []string) []string {
	wsRoot = filepath.Clean(wsRoot)
	if wsRoot == "" {
		return env
	}
	cache := envVal(env, "GOMODCACHE")
	if cache == "" {
		gopath := envVal(env, "GOPATH")
		if gopath == "" {
			gopath = defaultGoPath()
		}
		cache = filepath.Join(gopath, "pkg", "mod")
	}
	if !pathWithin(cache, wsRoot) {
		return env
	}
	alt := filepath.Join(os.TempDir(), "cogs-gomodcache")
	if home := strings.TrimSpace(os.Getenv("COGS_HOME")); home != "" && !pathWithin(home, wsRoot) {
		alt = filepath.Join(home, "go", "pkg", "mod")
	}
	return setEnv(env, "GOMODCACHE", alt)
}

func defaultGoPath() string {
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, "go")
	}
	return filepath.Join(os.TempDir(), "go")
}

func envVal(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimPrefix(e, prefix)
		}
	}
	if key == "GOMODCACHE" || key == "GOPATH" {
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
	}
	return ""
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

func pathWithin(child, parent string) bool {
	child = filepath.Clean(child)
	parent = filepath.Clean(parent)
	if child == parent {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
