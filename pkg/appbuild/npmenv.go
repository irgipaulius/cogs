package appbuild

import (
	"os"
	"os/exec"
	"strings"
)

// NpmEnv returns environment for npm commands, avoiding deprecated global npmrc keys.
func NpmEnv(extra []string) []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+4)
	for _, e := range env {
		if strings.HasPrefix(e, "npm_config_python=") {
			continue
		}
		out = append(out, e)
	}
	out = append(out, "NPM_CONFIG_GLOBALCONFIG="+os.DevNull)
	if py := npmPython(); py != "" {
		out = append(out, "PYTHON="+py, "npm_config_python="+py)
	}
	return append(out, extra...)
}

func npmPython() string {
	if v := strings.TrimSpace(os.Getenv("COGS_PYTHON")); v != "" {
		return v
	}
	for _, name := range []string{"python3", "python3.12", "python3.11", "python3.10"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}
