package version

import (
	"os/exec"
	"runtime/debug"
	"strings"
)

// Set at link time: -ldflags "-X github.com/irgipaulius/cogs/pkg/version.Version=v1.2.3 -X ...Commit=abc"
var (
	Version = "dev"
	Commit  = ""
)

func init() {
	if bi, ok := debug.ReadBuildInfo(); ok {
		if Version == "dev" {
			if v := strings.TrimSpace(bi.Main.Version); v != "" && v != "(devel)" {
				Version = v
			}
		}
		if Commit == "" {
			for _, s := range bi.Settings {
				if s.Key == "vcs.revision" && s.Value != "" {
					Commit = s.Value
					break
				}
			}
		}
	}
	if Version == "dev" {
		if out, err := exec.Command("git", "describe", "--tags", "--always").Output(); err == nil {
			if v := strings.TrimSpace(string(out)); v != "" {
				Version = v
			}
		}
	}
	if Commit == "" {
		if out, err := exec.Command("git", "rev-parse", "--short=12", "HEAD").Output(); err == nil {
			Commit = strings.TrimSpace(string(out))
		}
	}
	Version = strings.TrimSuffix(Version, "-dirty")
}

// String is the user-facing version line for `cogs version`.
func String() string {
	v := Version
	if v == "" {
		v = "dev"
	}
	v = strings.TrimSuffix(v, "-dirty")
	if Commit == "" {
		return v
	}
	short := Commit
	if len(short) > 12 {
		short = short[:12]
	}
	return v + " (" + short + ")"
}
