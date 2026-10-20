package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func wranglerCapture(args ...string) (string, error) {
	cmd, err := wranglerCmd(args...)
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

func wranglerCmd(args ...string) (*exec.Cmd, error) {
	if p, err := exec.LookPath("wrangler"); err == nil {
		return exec.Command(p, args...), nil
	}
	if _, err := exec.LookPath("npx"); err != nil {
		return nil, fmt.Errorf("wrangler not found; install: npm i -g wrangler")
	}
	return exec.Command("npx", append([]string{"wrangler"}, args...)...), nil
}

func ensureWranglerLogin() error {
	_, err := wranglerCapture("whoami")
	if err == nil {
		return nil
	}
	fmt.Println("Wrangler is not authenticated. Opening browser login…")
	if err := wrangler("login"); err != nil {
		return fmt.Errorf("wrangler login: %w", err)
	}
	_, err = wranglerCapture("whoami")
	return err
}

func wranglerTry(args ...string) error {
	_, err := wranglerCapture(args...)
	return err
}

func wrangler(args ...string) error {
	cmd, err := wranglerCmd(args...)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
