package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var forbiddenBins = map[string]struct{}{
	"sh": {}, "bash": {}, "zsh": {}, "ksh": {}, "fish": {}, "dash": {},
}

// RunCommandDriver executes argv with CoGS env vars (no shell).
func RunCommandDriver(ctx context.Context, appDir string, argv []string, env map[string]string) error {
	if len(argv) == 0 {
		return fmt.Errorf("empty argv")
	}
	binRel := argv[0]
	if strings.HasPrefix(binRel, "-") || strings.Contains(binRel, "-c") {
		return fmt.Errorf("invalid command argv")
	}
	bin, err := ResolveInsideApp(appDir, binRel)
	if err != nil {
		return err
	}
	base := strings.ToLower(filepath.Base(bin))
	if _, bad := forbiddenBins[base]; bad {
		return fmt.Errorf("shell execution not allowed")
	}
	st, err := os.Stat(bin)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("backup command must be a regular file")
	}
	cmd := exec.CommandContext(ctx, bin, argv[1:]...)
	cmd.Dir = appDir
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
