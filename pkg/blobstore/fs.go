package blobstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FS stores blobs on the local filesystem (host device).
type FS struct {
	Root string
}

func (f FS) safePath(key string) (string, error) {
	key = filepath.Clean(strings.ReplaceAll(key, "\\", "/"))
	if key == ".." || strings.HasPrefix(key, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid blob key")
	}
	return filepath.Join(f.Root, key), nil
}

func (f FS) Put(_ context.Context, key string, data []byte, _ string) error {
	p, err := f.safePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o640)
}

func (f FS) Get(_ context.Context, key string) ([]byte, error) {
	p, err := f.safePath(key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}
