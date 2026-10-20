package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Save writes cogs.launch.json (pretty, stable for git diffs).
func Save(dir string, m Manifest) error {
	m.applyDefaults()
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(filepath.Join(dir, DefaultFilename), b, 0o644)
}
