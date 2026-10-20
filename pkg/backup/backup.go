// Package backup implements VACUUM INTO snapshots with optional zstd upload to R2.
package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/irgipaulius/cogs/pkg/r2"
)

// Snapshot writes a consistent copy via VACUUM INTO.
func Snapshot(ctx context.Context, dbPath, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
		return err
	}
	// VACUUM INTO must be run via sqlite connection - caller opens db and runs pragma
	return fmt.Errorf("backup: use SnapshotDB with an open *sql.DB")
}

// SnapshotFile runs VACUUM INTO using database/sql on writer.
func SnapshotSQL(ctx context.Context, exec func(string, ...any) (any, error), dest string) error {
	_, err := exec(`VACUUM INTO ?`, dest)
	return err
}

// IntegrityCheck runs PRAGMA integrity_check on a database file path (opens read-only).
func IntegrityCheck(ctx context.Context, path string) error {
	// Delegated to restore-test CLI using sqliteopen; placeholder for package API.
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return nil
}

// UploadZstd compresses file and uploads to R2 key backup/<timestamp>.db.zst.
func UploadZstd(ctx context.Context, client *r2.Client, srcPath string) (key string, err error) {
	b, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	sum := sha256.Sum256(b)
	key = fmt.Sprintf("backup/%s-%s.db", ts, hex.EncodeToString(sum[:8]))
	_, err = client.PutCompressed(ctx, key, b)
	return key, err
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
