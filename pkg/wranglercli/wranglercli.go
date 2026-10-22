// Package wranglercli inspects Cloudflare wrangler login on the host.
package wranglercli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/irgipaulius/cogs/pkg/hostenv"
)

// AuthStatus is wrangler authentication on this host.
type AuthStatus struct {
	Installed bool   `json:"installed"`
	LoggedIn  bool   `json:"logged_in"`
	Email     string `json:"email,omitempty"`
	AccountID string `json:"account_id,omitempty"`
	Account   string `json:"account_name,omitempty"`
}

type whoamiJSON struct {
	Email    string `json:"email"`
	Accounts []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"accounts"`
}

func wranglerCmd(args ...string) (*exec.Cmd, error) {
	if p, err := exec.LookPath("wrangler"); err == nil {
		return exec.Command(p, args...), nil
	}
	if _, err := exec.LookPath("npx"); err != nil {
		return nil, fmt.Errorf("wrangler not found; run: npm i -g wrangler && wrangler login")
	}
	return exec.Command("npx", append([]string{"wrangler"}, args...)...), nil
}

func run(ctx context.Context, args ...string) (string, error) {
	cmd, err := wranglerCmd(args...)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Env = hostenv.CLIEnv(os.Getenv("COGS_WORKSPACE_ROOT"))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(out.String()))
	}
	return out.String(), nil
}

// Status returns wrangler whoami when logged in.
func Status(ctx context.Context) (AuthStatus, error) {
	st := AuthStatus{}
	if _, err := exec.LookPath("wrangler"); err != nil {
		if _, err2 := exec.LookPath("npx"); err2 != nil {
			return st, nil
		}
	}
	st.Installed = true
	raw, err := run(ctx, "whoami", "--json")
	if err != nil {
		return st, nil
	}
	var w whoamiJSON
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		return st, nil
	}
	st.LoggedIn = true
	st.Email = w.Email
	if len(w.Accounts) > 0 {
		st.AccountID = w.Accounts[0].ID
		st.Account = w.Accounts[0].Name
	}
	return st, nil
}
