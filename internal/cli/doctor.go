package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/irgipaulius/cogs/pkg/config"
	"github.com/irgipaulius/cogs/pkg/sqliteopen"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	envFile := fs.String("env", ".env", "env file")
	_ = fs.Parse(args)

	wd, err := os.Getwd()
	if err == nil {
		if root, ws, err := workspace.FindRoot(wd); err == nil && len(ws.Apps) > 0 {
			code := printDoctor(doctorWorkspace(root, ws))
			if code != 0 {
				return fmt.Errorf("doctor found problems")
			}
			return nil
		}
	}

	return doctorSingle(*envFile)
}

func doctorSingle(envFile string) error {
	cfg, err := config.LoadRelaxed(envFile)
	if err != nil {
		fmt.Println("config:", err)
	} else {
		fmt.Println("config: ok (relaxed)")
		fmt.Println("  listen:", cfg.ListenAddr)
		fmt.Println("  db:", cfg.DBPath)
		fmt.Println("  blobs:", cfg.BlobStore, cfg.BlobDir)
	}
	if cfg.BlobStore == "fs" && cfg.BlobDir != "" {
		if err := os.MkdirAll(cfg.BlobDir, 0o750); err != nil {
			fmt.Println("blob dir:", err)
		} else {
			fmt.Println("blob dir: writable", cfg.BlobDir)
		}
	}
	if _, err := os.Stat(cfg.DBPath); err == nil {
		p, err := sqliteopen.Open(sqliteopen.Config{Path: cfg.DBPath})
		if err != nil {
			fmt.Println("sqlite:", err)
		} else {
			mode, _ := sqliteopen.JournalMode(context.Background(), p.Reader)
			fmt.Println("sqlite: journal_mode=", mode)
			_ = p.Close()
		}
	}
	if cfg.PublicAPIURL != "" {
		fmt.Println("public API:", cfg.PublicAPIURL)
	}
	for _, bin := range []string{"wrangler", "cloudflared", "go", "npx"} {
		path, err := exec.LookPath(bin)
		if err != nil {
			fmt.Println(bin + ": not found")
		} else {
			fmt.Println(bin+":", path)
		}
	}
	return nil
}
