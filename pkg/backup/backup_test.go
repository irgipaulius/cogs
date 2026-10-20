package backup

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/irgipaulius/cogs/pkg/sqliteopen"
)

func TestIntegrityCheckFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.db")
	p, err := sqliteopen.Open(sqliteopen.Config{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = p.Writer.ExecContext(context.Background(), `CREATE TABLE t (id INT)`)
	_ = p.Close()
	if err := IntegrityCheck(context.Background(), path); err != nil {
		t.Fatal(err)
	}
}
