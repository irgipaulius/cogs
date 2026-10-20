package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func cmdDeps(args []string) error {
	fs := flag.NewFlagSet("deps", flag.ExitOnError)
	target := fs.String("target", runtime.GOOS+"/"+runtime.GOARCH, "GOOS/GOARCH")
	_ = fs.Parse(args)
	script := filepath.Join("tools", "build-deps", "build.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("run from a CoGS checkout; missing %s", script)
	}
	parts := splitTargets(*target)
	if len(parts) != 1 {
		return fmt.Errorf("deps: one --target at a time")
	}
	tg := parts[0]
	cmd := exec.Command("bash", script, tg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "."
	return cmd.Run()
}
