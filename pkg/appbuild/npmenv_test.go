package appbuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNpmEnvStripsDeprecatedPythonEnv(t *testing.T) {
	t.Setenv("npm_config_python", "/bad/global")
	env := NpmEnv(nil)
	for _, e := range env {
		if strings.EqualFold(e, "npm_config_python=/bad/global") {
			t.Fatalf("kept deprecated env python: %s", e)
		}
		if strings.HasPrefix(strings.ToLower(e), "npm_config_python=") {
			t.Fatalf("must not set npm_config_python: %s", e)
		}
	}
}

func TestNpmInstallNoUnknownPythonConfigWarning(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not installed")
	}
	dir := t.TempDir()
	pkg := `{"name":"cogs-npmenv-smoke","private":true,"version":"0.0.0"}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("npm", "install", "--ignore-scripts", "--no-audit", "--no-fund")
	cmd.Dir = dir
	cmd.Env = NpmEnv(nil)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("npm install: %v: %s", err, out)
	}
	if strings.Contains(strings.ToLower(string(out)), "unknown env config \"python\"") {
		t.Fatalf("npm warned about python config: %s", out)
	}
}
