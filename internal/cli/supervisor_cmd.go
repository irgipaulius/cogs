package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/supervisor"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func cmdSupervisor(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cogs supervisor run")
	}
	switch args[0] {
	case "run":
		return runSupervisor(args[1:])
	default:
		return fmt.Errorf("unknown supervisor subcommand %q", args[0])
	}
}

func runSupervisor(args []string) error {
	fs := flag.NewFlagSet("supervisor run", flag.ContinueOnError)
	wsRoot := fs.String("workspace", "", "workspace root (default: auto)")
	adminDB := fs.String("admin-db", "", "path to admin.db (default: apps/admin/data/admin.db)")
	_ = fs.Parse(args)

	root := *wsRoot
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		var err2 error
		root, _, err2 = workspace.FindRoot(wd)
		if err2 != nil {
			return err2
		}
	}
	dbPath := *adminDB
	if dbPath == "" {
		dbPath = filepath.Join(root, "apps", "admin", "data", "admin.db")
	}
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(root, dbPath)
	}

	srv, err := supervisor.NewServerFromWorkspace(root, dbPath)
	if err != nil {
		return err
	}
	defer srv.Store.Close()

	ux.OK("supervisor listening on " + srv.SocketPath)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()
	return srv.Run(ctx)
}

