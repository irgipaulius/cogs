package release

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveCommit maps a short or stale commit id to an on-disk release directory name.
func ResolveCommit(wsRoot, appID, commit string) string {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return ""
	}
	root := AppRoot(wsRoot, appID)
	if st, err := os.Stat(filepath.Join(root, commit)); err == nil && st.IsDir() {
		return commit
	}
	var matches []string
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == currentFile || name == previousFile {
			continue
		}
		if strings.HasPrefix(name, commit) || strings.HasPrefix(commit, name) {
			matches = append(matches, name)
		}
	}
	if len(matches) == 0 {
		return ""
	}
	sort.Slice(matches, func(i, j int) bool { return len(matches[i]) > len(matches[j]) })
	return matches[0]
}

// BinaryExists reports whether the release binary exists for commit (after resolve).
func BinaryExists(wsRoot, appID, commit string, combined bool) bool {
	return binaryExists(wsRoot, appID, commit, combined)
}

func binaryExists(wsRoot, appID, commit string, combined bool) bool {
	p := BinaryPath(wsRoot, appID, commit, combined)
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// BinaryForCommit returns the canonical commit dir name and binary path when present.
func BinaryForCommit(wsRoot, appID, commit string, combined bool) (resolvedCommit, bin string, ok bool) {
	resolved := ResolveCommit(wsRoot, appID, commit)
	if resolved == "" {
		return "", "", false
	}
	p := BinaryPath(wsRoot, appID, resolved, combined)
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		return "", "", false
	}
	return resolved, p, true
}

type releaseCandidate struct {
	commit string
	bin    string
	mod    int64
}

// BestRelease chooses the newest existing release binary (current pointer, then disk scan).
func BestRelease(wsRoot, appID string, combined bool) (commit, bin string, ok bool) {
	if cur, _ := ReadCurrent(wsRoot, appID); cur != "" {
		if c, p, ok := BinaryForCommit(wsRoot, appID, cur, combined); ok {
			return c, p, true
		}
	}
	root := AppRoot(wsRoot, appID)
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", "", false
	}
	var cands []releaseCandidate
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == currentFile || name == previousFile {
			continue
		}
		p := BinaryPath(wsRoot, appID, name, combined)
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		cands = append(cands, releaseCandidate{name, p, st.ModTime().Unix()})
	}
	if len(cands) == 0 {
		return "", "", false
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].mod > cands[j].mod })
	return cands[0].commit, cands[0].bin, true
}

// SyncPointer writes current to resolved commit when the binary exists.
func SyncPointer(wsRoot, appID, commit string, combined bool) (string, error) {
	c, _, ok := BinaryForCommit(wsRoot, appID, commit, combined)
	if !ok {
		return "", nil
	}
	if err := Activate(wsRoot, appID, c); err != nil {
		return "", err
	}
	return c, nil
}

// ListDiskCommits returns commit directory names that contain a binary.
func ListDiskCommits(wsRoot, appID string, combined bool) []string {
	root := AppRoot(wsRoot, appID)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == currentFile || name == previousFile {
			continue
		}
		if binaryExists(wsRoot, appID, name, combined) {
			out = append(out, name)
		}
	}
	return out
}
