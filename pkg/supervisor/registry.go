package supervisor

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

// AppDB describes a registered SQLite database for SQL console.
type AppDB struct {
	ID       string `json:"id"`
	AppName  string `json:"app_name"`
	Path     string `json:"-"`
	RelPath  string `json:"path,omitempty"`
}

// Registry lists workspace apps and their database paths.
type Registry struct {
	WorkspaceRoot string   `json:"workspace_root"`
	Apps          []AppDB  `json:"apps"`
}

// LoadRegistry builds db registry from workspace + per-app .env.
func LoadRegistry(wsRoot string) (Registry, error) {
	_, ws, err := workspace.FindRoot(wsRoot)
	if err != nil {
		return Registry{}, err
	}
	reg := Registry{WorkspaceRoot: wsRoot}
	for _, a := range ws.Apps {
		if a.Name == "admin" {
			continue
		}
		dir := filepath.Join(wsRoot, a.Path)
		dbPath := resolveDBPath(dir)
		if dbPath == "" {
			continue
		}
		abs, _ := filepath.Abs(dbPath)
		rel, _ := filepath.Rel(wsRoot, abs)
		reg.Apps = append(reg.Apps, AppDB{
			ID:      a.Name,
			AppName: a.Name,
			Path:    abs,
			RelPath: rel,
		})
	}
	return reg, nil
}

func resolveDBPath(appDir string) string {
	envPath := filepath.Join(appDir, ".env")
	if p := envGet(envPath, "COGS_DB_PATH"); p != "" {
		if !filepath.IsAbs(p) {
			p = filepath.Join(appDir, p)
		}
		return p
	}
	m, err := launch.Load(appDir)
	if err != nil {
		return ""
	}
	_ = m
	p := filepath.Join(appDir, "data", "todo.db")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return filepath.Join(appDir, "data.db")
}

func envGet(path, key string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	prefix := key + "="
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, prefix) {
			return strings.Trim(strings.TrimPrefix(line, prefix), `"'`)
		}
	}
	return ""
}
