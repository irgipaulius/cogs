package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveInsideApp returns an absolute path that must stay under appDir.
func ResolveInsideApp(appDir, rel string) (string, error) {
	appDir, err := filepath.Abs(appDir)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(appDir); err == nil {
		appDir = resolved
	}
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths not allowed")
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("path must not contain ..")
	}
	clean := filepath.Clean(rel)
	target := filepath.Join(appDir, clean)
	target, err = filepath.EvalSymlinks(target)
	if err != nil {
		if os.IsNotExist(err) {
			target = filepath.Join(appDir, clean)
		} else {
			return "", err
		}
	}
	appDir = filepath.Clean(appDir)
	target = filepath.Clean(target)
	if !pathWithin(target, appDir) {
		return "", fmt.Errorf("path escapes app directory")
	}
	return target, nil
}

func pathWithin(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// SnapshotRelPath returns a relative backup filename under backup_dir.
func SnapshotRelPath(backupDir, ts string) string {
	return filepath.ToSlash(filepath.Join(backupDir, ts+".db"))
}
