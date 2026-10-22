package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/ghcli"
	"github.com/irgipaulius/cogs/pkg/hostsetup"
	"github.com/irgipaulius/cogs/pkg/wranglercli"
)

func cmdHost(args []string) error {
	if len(args) == 0 || args[0] != "setup" {
		return fmt.Errorf("usage: cogs host setup")
	}
	if err := hostsetup.EnsureGitHubCLI(); err != nil {
		ux.Warn(err.Error())
	} else {
		ux.OK("GitHub CLI (gh) available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	st, _ := ghcli.Status(ctx)
	if !st.LoggedIn {
		fmt.Println("GitHub not logged in. Run on this host as the CoGS user:")
		for _, line := range hostsetup.GitHubLoginHint() {
			fmt.Println(" ", line)
		}
	} else {
		ux.OK("GitHub: " + st.Login)
	}
	cf, _ := wranglercli.Status(ctx)
	if !cf.LoggedIn {
		fmt.Println("Cloudflare: run: wrangler login")
	} else {
		ux.OK("Cloudflare: " + cf.Email)
	}
	if h := os.Getenv("COGS_HOME"); h != "" {
		fmt.Printf("COGS_HOME=%s (used for gh/wrangler config)\n", h)
	}
	return nil
}
