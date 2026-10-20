package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/irgipaulius/cogs/pkg/config"
	"github.com/irgipaulius/cogs/pkg/launch"
)

func explainBlobStorage(projectDir string, m launch.Manifest) {
	if !m.Attachments {
		return
	}
	abs, _ := filepath.Abs(projectDir)
	prefix := m.BlobKeyPrefix
	if prefix == "" {
		prefix = m.Name + "-files"
	}
	fmt.Printf("\n── %s attachments ──\n", m.Name)
	fmt.Println("  Stored on this host under COGS_BLOB_DIR (default: data/blobs).")
	fmt.Println("  SQLite holds metadata + blob keys only.")
	if m.R2Bucket != "" {
		fmt.Printf("  Optional edge sync: R2 bucket %q (cogs sync / COGS_BLOB_STORE=r2).\n", m.R2Bucket)
	}
	fmt.Printf("  Example path: %s/data/blobs/%s/<id>/…\n", abs, prefix)
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
