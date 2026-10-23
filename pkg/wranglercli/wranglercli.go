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
	"sync"
	"time"

	"github.com/irgipaulius/cogs/pkg/wranglerexec"
)

const statusFailBackoff = 5 * time.Minute

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

var statusCache struct {
	mu    sync.Mutex
	until time.Time
	st    AuthStatus
}

func run(ctx context.Context, args ...string) (string, error) {
	wsRoot := strings.TrimSpace(os.Getenv("COGS_WORKSPACE_ROOT"))
	cmd, err := wranglerexec.Command(wsRoot, args...)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(out.String()))
	}
	return out.String(), nil
}

// Status returns wrangler whoami when logged in (cached; backs off after failures).
func Status(ctx context.Context) (AuthStatus, error) {
	statusCache.mu.Lock()
	if time.Now().Before(statusCache.until) {
		st := statusCache.st
		statusCache.mu.Unlock()
		return st, nil
	}
	statusCache.mu.Unlock()

	st := AuthStatus{}
	if _, err := exec.LookPath("wrangler"); err != nil {
		if _, err2 := exec.LookPath("npx"); err2 != nil {
			return st, nil
		}
	}
	st.Installed = true
	raw, err := run(ctx, "whoami", "--json")
	if err != nil {
		statusCache.mu.Lock()
		statusCache.st = st
		statusCache.until = time.Now().Add(statusFailBackoff)
		statusCache.mu.Unlock()
		return st, nil
	}
	var w whoamiJSON
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		statusCache.mu.Lock()
		statusCache.until = time.Now().Add(statusFailBackoff)
		statusCache.mu.Unlock()
		return st, nil
	}
	st.LoggedIn = true
	st.Email = w.Email
	if len(w.Accounts) > 0 {
		st.AccountID = w.Accounts[0].ID
		st.Account = w.Accounts[0].Name
	}
	statusCache.mu.Lock()
	statusCache.st = st
	if st.LoggedIn {
		statusCache.until = time.Now().Add(2 * time.Minute)
	} else {
		statusCache.until = time.Now().Add(statusFailBackoff)
	}
	statusCache.mu.Unlock()
	return st, nil
}
