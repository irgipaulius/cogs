package gitconfig

import (
	"context"
	"os/exec"

	"github.com/irgipaulius/cogs/pkg/hostenv"
)

// EnsureSafeDirectory registers dir so git will operate when ownership differs from the current user.
func EnsureSafeDirectory(ctx context.Context, wsRoot, dir string) error {
	if dir == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "git", "config", "--global", "--add", "safe.directory", dir)
	cmd.Env = hostenv.CLIEnv(wsRoot)
	if out, err := cmd.CombinedOutput(); err != nil {
		return errOutput(err, out)
	}
	return nil
}

func errOutput(err error, out []byte) error {
	if len(out) == 0 {
		return err
	}
	return &execError{err: err, out: string(out)}
}

type execError struct {
	err error
	out string
}

func (e *execError) Error() string {
	return e.err.Error() + ": " + e.out
}

func (e *execError) Unwrap() error { return e.err }
