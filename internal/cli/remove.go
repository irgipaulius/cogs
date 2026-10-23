package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/irgipaulius/cogs/internal/cli/ux"
	"github.com/irgipaulius/cogs/pkg/reconcile"
	"github.com/irgipaulius/cogs/pkg/state"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func cmdRemove(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: cogs remove <app>")
	}
	name := strings.TrimSpace(args[0])
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, _, err := workspace.FindRoot(wd)
	if err != nil {
		return err
	}
	st, err := state.Open(root)
	if err != nil {
		return err
	}
	defer st.Close()
	app, err := st.GetApp(name)
	if err != nil {
		return fmt.Errorf("app %q not in state: %w", name, err)
	}
	d := app.AppDesired
	d.Present = false
	if err := st.SetDesired(d); err != nil {
		return err
	}
	ctx := context.Background()
	if err := reconcile.Run(ctx, st, name, reconcile.Options{}); err != nil {
		return err
	}
	ux.OK("removed " + name)
	return nil
}
