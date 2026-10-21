// Package release tracks backend build directories and current/previous pointers (text files, Windows-safe).
package release

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	currentFile  = "current"
	previousFile = "previous"
)

// AppRoot is .cogs/releases/<appID> under the workspace.
func AppRoot(wsRoot, appID string) string {
	return filepath.Join(wsRoot, ".cogs", "releases", appID)
}

func commitDir(wsRoot, appID, commit string) string {
	return filepath.Join(AppRoot(wsRoot, appID), strings.TrimSpace(commit))
}

// BinaryName returns the platform executable name for an app binary.
func BinaryName(combined bool) string {
	if combined {
		if runtime.GOOS == "windows" {
			return "cogs-admin.exe"
		}
		return "cogs-admin"
	}
	if runtime.GOOS == "windows" {
		return "cogs-app.exe"
	}
	return "cogs-app"
}

// BinaryPath is the built backend executable for a commit.
func BinaryPath(wsRoot, appID, commit string, combined bool) string {
	return filepath.Join(commitDir(wsRoot, appID, commit), BinaryName(combined))
}

// ReadCurrent returns the active commit hash, if any.
func ReadCurrent(wsRoot, appID string) (string, error) {
	b, err := os.ReadFile(filepath.Join(AppRoot(wsRoot, appID), currentFile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// ReadPrevious returns the previous commit hash, if any.
func ReadPrevious(wsRoot, appID string) (string, error) {
	b, err := os.ReadFile(filepath.Join(AppRoot(wsRoot, appID), previousFile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// Activate sets current to commit and moves the old current to previous.
func Activate(wsRoot, appID, commit string) error {
	root := AppRoot(wsRoot, appID)
	if err := os.MkdirAll(root, 0o750); err != nil {
		return err
	}
	cur, _ := ReadCurrent(wsRoot, appID)
	if cur != "" && cur != commit {
		if err := os.WriteFile(filepath.Join(root, previousFile), []byte(cur+"\n"), 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(root, currentFile), []byte(commit+"\n"), 0o644)
}

// CurrentBinary returns the path to the live release binary when current is set.
func CurrentBinary(wsRoot, appID string, combined bool) (string, bool) {
	commit, err := ReadCurrent(wsRoot, appID)
	if err != nil || commit == "" {
		return "", false
	}
	p := BinaryPath(wsRoot, appID, commit, combined)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p, true
	}
	return "", false
}
