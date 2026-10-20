package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/irgipaulius/cogs/pkg/launch"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func cmdApps(args []string) error {
	if len(args) > 0 && args[0] != "list" {
		return fmt.Errorf("usage: cogs apps list")
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	rootDir, ws, err := workspace.FindRoot(wd)
	if err != nil {
		return fmt.Errorf("no %s found above %s", workspace.FileName, wd)
	}
	fmt.Printf("workspace %q at %s\n\n", ws.Name, rootDir)
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tLISTEN\tPATH\tDESCRIPTION")
	for _, a := range ws.Apps {
		desc := a.Description
		if desc == "" {
			desc = "—"
		}
		dir := filepath.Join(rootDir, a.Path)
		m, err := launch.Load(dir)
		listen := "—"
		if err == nil {
			resolveListen(dir, &m)
			listen = listenAddr(m.BindHost, m.Port)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", a.Name, listen, a.Path, desc)
	}
	return tw.Flush()
}
