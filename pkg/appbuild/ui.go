package appbuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const defaultFrontendDir = "frontend"

// Build runs npm install (when needed) and npm run build for frontend/ or project root package.json.
func Build(projectDir string, extraEnv []string, quick bool) error {
	frontDir := filepath.Join(projectDir, defaultFrontendDir)
	pkg := filepath.Join(frontDir, "package.json")
	buildDir := projectDir
	if _, err := os.Stat(pkg); err != nil {
		pkg = filepath.Join(projectDir, "package.json")
		buildDir = projectDir
	} else {
		buildDir = frontDir
	}
	if _, err := os.Stat(pkg); err != nil {
		return nil
	}
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found — install Node.js to build UI in %s", buildDir)
	}
	nodeModules := filepath.Join(buildDir, "node_modules")
	if !quick || nodeModulesMissing(nodeModules) {
		if err := npm(buildDir, extraEnv, "install"); err != nil {
			return fmt.Errorf("npm install: %w", err)
		}
	}
	staging := PagesDistDir(projectDir)
	env := append(extraEnv, "COGS_VITE_OUT_DIR="+staging)
	if err := npm(buildDir, env, "run", "build"); err != nil {
		return fmt.Errorf("npm run build: %w", err)
	}
	return nil
}

func nodeModulesMissing(dir string) bool {
	_, err := os.Stat(dir)
	return err != nil
}

func npm(dir string, extraEnv []string, args ...string) error {
	cmd := exec.Command("npm", args...)
	cmd.Dir = dir
	cmd.Env = NpmEnv(extraEnv)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
