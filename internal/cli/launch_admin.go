package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func ensureAdminEnv(path string, m launch.Manifest, publicHost string) error {
	wsRoot := filepath.Dir(filepath.Dir(path))
	if root, _, err := workspace.FindRoot(path); err == nil {
		wsRoot = root
	}
	origin := "https://" + publicHost
	if _, err := os.Stat(path); err == nil {
		ux.Info("Using existing admin .env")
		return appendEnvKeys(path, map[string]string{
			"COGS_LISTEN":            listenAddr(m.BindHost, m.Port),
			"COGS_PUBLIC_PAGE_ORIGIN": origin,
			"COGS_PUBLIC_API_URL":     origin,
			"COGS_WORKSPACE_ROOT":     wsRoot,
			"COGS_ADMIN_DB_PATH":      "data/admin.db",
			"COGS_ALLOW_PUBLIC_BIND":  "1",
		})
	}
	content := fmt.Sprintf(`COGS_LISTEN=%s
COGS_ALLOW_PUBLIC_BIND=1
COGS_ADMIN_DB_PATH=data/admin.db
COGS_WORKSPACE_ROOT=%s
COGS_PUBLIC_PAGE_ORIGIN=%s
COGS_PUBLIC_API_URL=%s
`, listenAddr(m.BindHost, m.Port), wsRoot, origin, origin)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}
	fmt.Println("Wrote", path)
	return nil
}
