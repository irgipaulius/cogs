package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/irgipaulius/cogs/pkg/config"
)

func explainBlobStorage(projectDir string) {
	abs, _ := filepath.Abs(projectDir)
	fmt.Println("\n── Todo attachments (images) ──")
	fmt.Println("  Stored on this host under COGS_BLOB_DIR (default: data/blobs).")
	fmt.Println("  SQLite holds metadata + file keys only.")
	fmt.Println("  Cloudflare R2 is optional (COGS_BLOB_STORE=r2 + credentials) for edge sync, not required for uploads.")
	fmt.Printf("  Example path: %s/data/blobs/todo-images/<id>/…\n", abs)
}

func ensureBlobStorage(projectDir string) error {
	envPath := filepath.Join(projectDir, ".env")
	cfg, err := config.Load(envPath)
	if err != nil {
		return err
	}
	dir := cfg.BlobDir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(projectDir, dir)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create blob dir %s: %w", dir, err)
	}
	return nil
}
