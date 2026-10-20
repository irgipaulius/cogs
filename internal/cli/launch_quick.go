package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

func ensureGoModTidy(projectDir string) error {
	if _, err := os.Stat(filepath.Join(projectDir, "go.mod")); err != nil {
		return nil
	}
	fmt.Println("→ go mod tidy")
	return runGo(projectDir, "mod", "tidy")
}
