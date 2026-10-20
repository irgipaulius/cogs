package sqliteopen

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenWAL(t *testing.T) {
	dir := t.TempDir()
	p, err := Open(Config{Path: filepath.Join(dir, "test.db"), ReaderConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()
	if err := Health(ctx, p.Reader); err != nil {
		t.Fatal(err)
	}
	mode, err := JournalMode(ctx, p.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode=%q want wal", mode)
	}
}
