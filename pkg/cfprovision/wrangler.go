package cfprovision

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/irgipaulius/cogs/pkg/hostenv"
)

func wranglerCmd(wsRoot string, args ...string) (*exec.Cmd, error) {
	if p, err := exec.LookPath("wrangler"); err == nil {
		cmd := exec.Command(p, args...)
		cmd.Env = hostenv.CLIEnv(wsRoot)
		return cmd, nil
	}
	if _, err := exec.LookPath("npx"); err != nil {
		return nil, fmt.Errorf("wrangler not found; install: npm i -g wrangler")
	}
	cmd := exec.Command("npx", append([]string{"wrangler"}, args...)...)
	cmd.Env = hostenv.CLIEnv(wsRoot)
	return cmd, nil
}

func wranglerTryCreate(wsRoot string, args ...string) error {
	out, err := wranglerCaptureBoth(wsRoot, args...)
	if err == nil {
		return nil
	}
	if wranglerErrorBenign(out) {
		return nil
	}
	return fmt.Errorf("wrangler %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
}

func wranglerCaptureBoth(wsRoot string, args ...string) ([]byte, error) {
	cmd, err := wranglerCmd(wsRoot, args...)
	if err != nil {
		return nil, err
	}
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	err = cmd.Run()
	return combined.Bytes(), err
}

func wranglerErrorBenign(out []byte) bool {
	s := strings.ToLower(string(out))
	phrases := []string{
		"already exists",
		"already have a tunnel",
		"database with that name already exists",
		"bucket you tried to create already exists",
	}
	for _, p := range phrases {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
