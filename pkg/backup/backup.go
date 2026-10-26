// Package backup implements per-app database snapshots and restore.
package backup

import (
	"context"
	"io"
	"os"
)

// IntegrityCheck runs PRAGMA integrity_check on a database file path.
func IntegrityCheck(ctx context.Context, path string) error {
	return IntegrityCheckFile(ctx, path)
}

// CopyFile is a test helper.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
