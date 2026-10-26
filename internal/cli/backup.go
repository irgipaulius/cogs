package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/backup"
	"github.com/irgipaulius/cogs/pkg/state"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func cmdBackup(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: cogs backup now|list|restore <app> [id] [--yes]")
	}
	switch args[0] {
	case "now":
		return cmdBackupNow(args[1:])
	case "list":
		return cmdBackupList(args[1:])
	case "restore":
		return cmdBackupRestore(args[1:])
	default:
		return fmt.Errorf("usage: cogs backup now|list|restore <app> [id] [--yes]")
	}
}

func cmdBackupNow(args []string) error {
	appID, root, dir, st, err := backupAppContext(args)
	if err != nil {
		return err
	}
	defer st.Close()
	svc := &backup.Service{WSRoot: root, Store: st}
	id, err := svc.Snapshot(context.Background(), appID, dir)
	if err != nil {
		return err
	}
	ux.OK(fmt.Sprintf("%s backup %d created", appID, id))
	return nil
}

func cmdBackupList(args []string) error {
	appID, _, _, st, err := backupAppContext(args)
	if err != nil {
		return err
	}
	defer st.Close()
	rows, err := st.ListBackups(appID, 50)
	if err != nil {
		return err
	}
	for _, r := range rows {
		fmt.Printf("%d  %s  %d bytes  %s\n", r.ID, r.CreatedAt.UTC().Format(time.RFC3339), r.Bytes, r.RelPath)
	}
	return nil
}

func cmdBackupRestore(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: cogs backup restore <app> <id> --yes")
	}
	if !globals.Yes {
		return fmt.Errorf("restore is destructive — pass --yes")
	}
	rest := args
	appID := rest[0]
	id, err := strconv.ParseInt(rest[1], 10, 64)
	if err != nil {
		return err
	}
	_, root, dir, st, err := backupAppContext([]string{appID})
	if err != nil {
		return err
	}
	defer st.Close()
	svc := &backup.Service{WSRoot: root, Store: st}
	stop := func() error {
		stopApp(dir)
		return nil
	}
	start := func() error {
		if err := ensureReleaseRunningQuiet(dir); err != nil {
			if strings.Contains(err.Error(), "no release binary on disk") {
				return nil
			}
			return err
		}
		return nil
	}
	if err := svc.Restore(context.Background(), appID, dir, id, stop, start); err != nil {
		return err
	}
	ux.OK(fmt.Sprintf("%s restored from backup %d", appID, id))
	return nil
}

func backupAppContext(args []string) (appID, root, dir string, st *state.Store, err error) {
	if len(args) < 1 {
		err = fmt.Errorf("app name required")
		return
	}
	appID = args[0]
	wd, e := os.Getwd()
	if e != nil {
		err = e
		return
	}
	root, ws, e := workspace.FindRoot(wd)
	if e != nil {
		err = e
		return
	}
	dir, e = workspace.ResolveApp(root, ws, appID)
	if e != nil {
		err = e
		return
	}
	if _, e = appcfg.Load(dir); e != nil {
		err = e
		return
	}
	st, err = state.Open(root)
	return
}

func runScheduledBackups(root string, ws workspace.Root, st *state.Store, quiet bool) {
	if st == nil {
		return
	}
	svc := &backup.Service{WSRoot: root, Store: st}
	now := time.Now().UTC()
	for _, a := range ws.Apps {
		dir, err := workspace.ResolveApp(root, ws, a.Name)
		if err != nil {
			continue
		}
		app, err := appcfg.Load(dir)
		if err != nil {
			continue
		}
		cfg, err := app.NormalizeBackup()
		if err != nil || !cfg.Enabled {
			continue
		}
		last, _ := st.LatestBackupTime(a.Name)
		if !backup.Due(last, cfg.Every, now) {
			continue
		}
		if _, err := svc.Snapshot(context.Background(), a.Name, dir); err != nil {
			if !quiet {
				ux.Warn(a.Name + " backup: " + err.Error())
			}
		} else if !quiet {
			ux.Info(a.Name + ": scheduled backup ok")
		}
	}
}
