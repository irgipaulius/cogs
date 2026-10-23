package appremove

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/irgipaulius/cogs/pkg/gowork"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

// Options controls removing an installed app from the workspace overlay.
type Options struct {
	WSRoot       string
	AppName      string
	RemoveClone  bool
	RemoveGoWork bool
}

// Remove unregisters the app and optionally drops clone + go.work entry.
func Remove(ctx context.Context, opts Options) (detail string, err error) {
	name := strings.TrimSpace(opts.AppName)
	if name == "" {
		return "", fmt.Errorf("app name required")
	}
	wsRoot, err := filepath.Abs(opts.WSRoot)
	if err != nil {
		return "", err
	}
	ia, ok := workspace.InstalledLookup(wsRoot, name)
	if !ok {
		return "", fmt.Errorf("app %q is not an installed overlay app", name)
	}
	relPath := ia.App.Path
	if err := workspace.RemoveInstalledApp(wsRoot, name); err != nil {
		return "", err
	}
	if opts.RemoveGoWork && relPath != "" {
		_ = gowork.RemoveModule(wsRoot, relPath)
	}
	if opts.RemoveClone && strings.TrimSpace(ia.GitRoot) != "" {
		clone := filepath.Join(wsRoot, filepath.FromSlash(ia.GitRoot))
		_ = os.RemoveAll(clone)
	}
	return fmt.Sprintf("removed %s from installed overlay", name), nil
}
