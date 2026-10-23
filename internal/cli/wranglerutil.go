package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/irgipaulius/cogs/pkg/workspace"
	"github.com/irgipaulius/cogs/pkg/wranglerexec"
)

func cliWSRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	root, _, err := workspace.FindRoot(wd)
	if err != nil {
		return wd
	}
	return root
}

func wranglerCapture(wsRoot string, args ...string) (string, error) {
	cmd, err := wranglerCmd(wsRoot, args...)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func wranglerCmd(wsRoot string, args ...string) (*exec.Cmd, error) {
	if wsRoot == "" {
		wsRoot = cliWSRoot()
	}
	return wranglerexec.Command(wsRoot, args...)
}

func ensureWranglerLogin(wsRoot string) error {
	if wsRoot == "" {
		wsRoot = cliWSRoot()
	}
	_, err := wranglerCapture(wsRoot, "whoami")
	if err == nil {
		return nil
	}
	fmt.Println("Set COGS_CF_API_TOKEN or run wrangler login…")
	if err := wrangler(wsRoot, "login"); err != nil {
		return fmt.Errorf("wrangler login: %w", err)
	}
	_, err = wranglerCapture(wsRoot, "whoami")
	return err
}

func wrangler(wsRoot string, args ...string) error {
	if wsRoot == "" {
		wsRoot = cliWSRoot()
	}
	cmd, err := wranglerCmd(wsRoot, args...)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
