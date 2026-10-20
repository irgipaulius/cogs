package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const defaultFrontendDir = "frontend"

// buildLaunchFrontend runs npm install + build when frontend/package.json exists.
func buildLaunchFrontend(projectDir string, quick bool) error {
	frontDir := filepath.Join(projectDir, defaultFrontendDir)
	pkg := filepath.Join(frontDir, "package.json")
	if _, err := os.Stat(pkg); err != nil {
		return nil
	}
	fmt.Println("→ UI build (npm run build → public/)")
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found — install Node.js or build UI manually in %s", frontDir)
	}
	nodeModules := filepath.Join(frontDir, "node_modules")
	if !quick {
		if err := npm(frontDir, "install"); err != nil {
			return fmt.Errorf("npm install: %w", err)
		}
	} else if _, err := os.Stat(nodeModules); err != nil {
		if err := npm(frontDir, "install"); err != nil {
			return fmt.Errorf("npm install: %w", err)
		}
	}
	if err := npm(frontDir, "run", "build"); err != nil {
		return fmt.Errorf("npm run build: %w", err)
	}
	return nil
}

func npm(dir string, args ...string) error {
	cmd := exec.Command("npm", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
