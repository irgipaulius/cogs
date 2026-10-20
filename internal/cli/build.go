package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func cmdBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	targets := fs.String("target", "", "comma-separated GOOS/GOARCH pairs, e.g. linux/amd64,freebsd/arm64")
	out := fs.String("o", "", "output binary path (single target only)")
	_ = fs.Parse(args)

	tgs := splitTargets(*targets)
	if len(tgs) == 0 {
		tgs = []string{runtime.GOOS + "/" + runtime.GOARCH}
	}
	for _, tg := range tgs {
		parts := strings.Split(tg, "/")
		if len(parts) != 2 {
			return fmt.Errorf("bad target %q", tg)
		}
		goos, goarch := parts[0], parts[1]
		bin := *out
		if bin == "" {
			bin = filepath.Join("dist", goos+"_"+goarch, "app")
		}
		if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
			return err
		}
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", bin, "./cmd/app")
		cmd.Env = append(os.Environ(),
			"CGO_ENABLED=0",
			"GOOS="+goos,
			"GOARCH="+goarch,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("building %s/%s -> %s\n", goos, goarch, bin)
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}
