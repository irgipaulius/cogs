package fileutil

import (
	"bytes"
	"os"
)

// WriteFileIfChanged writes path only when content differs (mode preserved when file exists).
func WriteFileIfChanged(path string, content []byte, mode os.FileMode) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, content) {
		return nil
	}
	return os.WriteFile(path, content, mode)
}
