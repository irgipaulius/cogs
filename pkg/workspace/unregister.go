package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// UnregisterApp removes an app from installed overlay and tracked workspace manifest.
func UnregisterApp(wsRoot, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return os.ErrInvalid
	}
	_ = RemoveInstalledApp(wsRoot, name)
	path := filepath.Join(wsRoot, FileName)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var r Root
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	out := r.Apps[:0]
	for _, a := range r.Apps {
		if a.Name != name {
			out = append(out, a)
		}
	}
	if len(out) == len(r.Apps) {
		return nil
	}
	r.Apps = out
	nb, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	nb = append(nb, '\n')
	return os.WriteFile(path, nb, 0o644)
}
