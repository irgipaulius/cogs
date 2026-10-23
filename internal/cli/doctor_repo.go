package cli

import (
	"os"
	"path/filepath"
	"strings"
)

func doctorRepoJunk(root string) []doctorFinding {
	var out []doctorFinding
	if root == "" {
		return out
	}
	if ent, err := os.Stat(filepath.Join(root, "1")); err == nil && !ent.IsDir() {
		out = append(out, doctorFinding{
			OK: false, Problem: "stray file at repo root: 1",
			Fix: "rm " + filepath.Join(root, "1"),
		})
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		if strings.Contains(name, ".bak") {
			out = append(out, doctorFinding{
				OK: false, Problem: "stray backup file at repo root: " + name,
				Fix: "rm " + filepath.Join(root, name),
			})
		}
	}
	return out
}
