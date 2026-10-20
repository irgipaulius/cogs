package geo

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/irgipaulius/cogs/pkg/sqliteopen"
)

func TestDetectModeAndBBox(t *testing.T) {
	p, err := sqliteopen.Open(sqliteopen.Config{Path: filepath.Join(t.TempDir(), "g.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()
	mode := DetectMode(ctx, p.Writer)
	_, err = p.Writer.ExecContext(ctx, `CREATE TABLE geo_points (id INTEGER PRIMARY KEY, lat REAL, lon REAL, name TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, p.Writer, mode); err != nil {
		t.Fatal(err)
	}
	_, _ = p.Writer.ExecContext(ctx, `INSERT INTO geo_points (id, lat, lon, name) VALUES (1, 10, 20, 'a')`)
	if mode == ModeRTree {
		tx, _ := p.Writer.BeginTx(ctx, nil)
		_ = UpsertRTree(ctx, tx, 1, 20, 10)
		_ = tx.Commit()
	}
	ids, err := QueryBBox(ctx, p.Reader, mode, 19, 9, 21, 11)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) < 1 {
		t.Fatalf("expected hits, mode=%s", mode)
	}
}
