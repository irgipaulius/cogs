package cli

import (
	"flag"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/config"
	"github.com/irgipaulius/cogs/pkg/launch"
)

func cmdPing(args []string) error {
	fs := flag.NewFlagSet("ping", flag.ExitOnError)
	dir := fs.String("dir", ".", "app directory")
	urlOverride := fs.String("url", "", "override health URL")
	link := fs.Bool("link", false, "also hit /link/ping")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if rest := fs.Args(); len(rest) > 0 {
		*dir = rest[0]
	}
	abs, err := resolveLaunchProjectDir(*dir)
	if err != nil {
		return err
	}
	base := *urlOverride
	if base == "" {
		envPath := filepath.Join(abs, ".env")
		if cfg, err := config.Load(envPath); err == nil && cfg.ListenAddr != "" {
			base = "http://" + cfg.ListenAddr
		} else {
			m, _ := launch.Load(abs)
			resolveListen(abs, &m)
			base = "http://" + listenAddr(m.BindHost, m.Port)
		}
	}
	base = stringsTrimRightSlash(base)
	if globals.DryRun {
		ux.Plan("ping dry-run", []string{base + "/health"})
		return nil
	}
	client := &http.Client{Timeout: 5 * time.Second}
	if err := pingURL(client, base+"/health"); err != nil {
		return err
	}
	ux.OK("health " + base + "/health")
	if *link {
		if err := pingURL(client, base+"/link/ping"); err != nil {
			return err
		}
		ux.OK("link " + base + "/link/ping")
	}
	return nil
}

func pingURL(client *http.Client, url string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s → %d", url, resp.StatusCode)
	}
	return nil
}

func stringsTrimRightSlash(s string) string {
	for len(s) > 1 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
