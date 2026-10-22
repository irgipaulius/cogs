package gowork

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const relPath = ".cogs/go.work"

// Path returns absolute GOWORK path when the file exists or can be created.
func Path(wsRoot string) string {
	if wsRoot == "" {
		return ""
	}
	return filepath.Join(wsRoot, relPath)
}

// AppendEnv sets GOWORK when .cogs/go.work exists.
func AppendEnv(wsRoot string, env []string) []string {
	p := Path(wsRoot)
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return env
	}
	key := "GOWORK="
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if strings.HasPrefix(e, key) {
			continue
		}
		out = append(out, e)
	}
	return append(out, key+p)
}

// EnsureModule adds a module path to .cogs/go.work (never touches tracked go.work).
func EnsureModule(wsRoot, moduleRel string) error {
	wsRoot, err := filepath.Abs(wsRoot)
	if err != nil {
		return err
	}
	moduleRel = filepath.ToSlash(strings.TrimSpace(moduleRel))
	if moduleRel == "" {
		return nil
	}
	usePath := "./" + strings.TrimPrefix(moduleRel, "./")
	if !strings.HasPrefix(usePath, "./") {
		usePath = "./" + usePath
	}
	modDir := filepath.Join(wsRoot, moduleRel)
	if _, err := os.Stat(filepath.Join(modDir, "go.mod")); err != nil {
		return nil
	}

	p := Path(wsRoot)
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	existing := readUses(p)
	if containsUse(existing, usePath) {
		return nil
	}
	existing = append(existing, usePath)
	if err := writeUses(p, existing); err != nil {
		return err
	}
	cmd := exec.Command("go", "work", "sync")
	cmd.Dir = wsRoot
	cmd.Env = AppendEnv(wsRoot, os.Environ())
	_, _ = cmd.CombinedOutput()
	return nil
}

// SyncFromRepo copies use paths from tracked go.work into .cogs/go.work when missing.
func SyncFromRepo(wsRoot string) error {
	wsRoot, err := filepath.Abs(wsRoot)
	if err != nil {
		return err
	}
	tracked := filepath.Join(wsRoot, "go.work")
	uses := readUses(tracked)
	if len(uses) == 0 {
		return nil
	}
	p := Path(wsRoot)
	merged := readUses(p)
	for _, u := range uses {
		if !containsUse(merged, u) {
			merged = append(merged, u)
		}
	}
	return writeUses(p, merged)
}

func readUses(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var uses []string
	inUse := false
	for _, line := range strings.Split(string(b), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "use (" {
			inUse = true
			continue
		}
		if inUse && trim == ")" {
			break
		}
		if inUse && strings.HasPrefix(trim, "./") {
			uses = append(uses, trim)
		}
	}
	return uses
}

func writeUses(path string, uses []string) error {
	if len(uses) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var uniq []string
	for _, u := range uses {
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		uniq = append(uniq, u)
	}
	var b strings.Builder
	b.WriteString("go 1.23\n\nuse (\n")
	for _, u := range uniq {
		b.WriteString("\t")
		b.WriteString(u)
		b.WriteString("\n")
	}
	b.WriteString(")\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func containsUse(uses []string, path string) bool {
	for _, u := range uses {
		if u == path {
			return true
		}
	}
	return false
}

// InitBestEffort ensures .cogs/go.work exists by seeding from repo go.work.
func InitBestEffort(wsRoot string) error {
	p := Path(wsRoot)
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	if err := SyncFromRepo(wsRoot); err != nil {
		return err
	}
	return nil
}

// ValidateModuleInWork returns an error when go.mod exists but module is not in GOWORK.
func ValidateModuleInWork(wsRoot, moduleRel string) error {
	mod := filepath.Join(wsRoot, moduleRel, "go.mod")
	if _, err := os.Stat(mod); err != nil {
		return nil
	}
	uses := readUses(Path(wsRoot))
	usePath := "./" + filepath.ToSlash(strings.TrimPrefix(moduleRel, "./"))
	if !containsUse(uses, usePath) {
		return fmt.Errorf("module %s not in %s — run: cogs doctor", usePath, relPath)
	}
	return nil
}
