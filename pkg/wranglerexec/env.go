// Package wranglerexec runs wrangler with CoGS Cloudflare token env (not OAuth-dependent).
package wranglerexec

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/cftoken"
	"github.com/irgipaulius/cogs/pkg/hostenv"
)

// Env returns environment for supervised wrangler subprocesses.
func Env(wsRoot string) ([]string, error) {
	base := hostenv.CLIEnv(wsRoot)
	if wsRoot != "" {
		logDir := filepath.Join(wsRoot, ".cogs", "run")
		_ = os.MkdirAll(logDir, 0o755)
		base = setEnv(base, "WRANGLER_LOG", "error")
		base = setEnv(base, "WRANGLER_LOG_PATH", filepath.Join(logDir, "wrangler.log"))
	}
	res := cftoken.Resolver{WSRoot: wsRoot, OAuth: cftoken.WranglerOAuthToken}
	tok, err := res.For(context.Background(), cftoken.ScopeAPI)
	if err != nil {
		return base, err
	}
	base = setEnv(base, "CLOUDFLARE_API_TOKEN", tok)
	if id := accountIDForWrangler(wsRoot); id != "" {
		base = setEnv(base, "CLOUDFLARE_ACCOUNT_ID", id)
	}
	return base, nil
}

func accountIDForWrangler(wsRoot string) string {
	if id := strings.TrimSpace(os.Getenv("COGS_CF_ACCOUNT_ID")); id != "" {
		return id
	}
	if wsRoot == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(wsRoot, ".cogs", "cloudflare", "account_id"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// Command builds wrangler (or npx wrangler) with Env applied.
func Command(wsRoot string, args ...string) (*exec.Cmd, error) {
	cmd, err := lookWrangler(args...)
	if err != nil {
		return nil, err
	}
	env, err := Env(wsRoot)
	if err != nil {
		return nil, err
	}
	cmd.Env = env
	return cmd, nil
}

func lookWrangler(args ...string) (*exec.Cmd, error) {
	if p, err := exec.LookPath("wrangler"); err == nil {
		return exec.Command(p, args...), nil
	}
	if _, err := exec.LookPath("npx"); err != nil {
		return nil, exec.ErrNotFound
	}
	return exec.Command("npx", append([]string{"wrangler"}, args...)...), nil
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
