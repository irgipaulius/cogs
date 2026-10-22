package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const FileName = "cogs.workspace.json"

// Root describes the monorepo orchestrator layout.
type Root struct {
	Name   string `json:"name"`
	Deploy Deploy `json:"deploy"`
	Apps   []App  `json:"apps"`
}

// App is one deployable unit under apps/.
type App struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
}

// FindRoot walks up from startDir looking for cogs.workspace.json.
func FindRoot(startDir string) (rootDir string, root Root, err error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", Root{}, err
	}
	for {
		p := filepath.Join(dir, FileName)
		if b, err := os.ReadFile(p); err == nil {
			var r Root
			if err := json.Unmarshal(b, &r); err != nil {
				return "", Root{}, err
			}
			r.ApplyDeployDefaults()
			r = MergeInstalled(dir, r)
			return dir, r, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", Root{}, os.ErrNotExist
		}
		dir = parent
	}
}

// ResolveApp returns absolute path for an app name or relative path.
func ResolveApp(wsRoot string, ws Root, nameOrPath string) (string, error) {
	if filepath.IsAbs(nameOrPath) {
		return nameOrPath, nil
	}
	for _, a := range ws.Apps {
		if a.Name == nameOrPath || a.Path == nameOrPath {
			return filepath.Join(wsRoot, a.Path), nil
		}
	}
	candidate := filepath.Join(wsRoot, nameOrPath)
	if _, err := os.Stat(filepath.Join(candidate, "cogs.launch.json")); err == nil {
		return candidate, nil
	}
	return candidate, nil
}
