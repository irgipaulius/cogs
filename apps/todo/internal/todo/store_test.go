package todo

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/irgipaulius/cogs/apps/todo/internal/db"
	"github.com/irgipaulius/cogs/pkg/sqliteopen"
)

func TestUpsertAndList(t *testing.T) {
	p, err := sqliteopen.Open(sqliteopen.Config{Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()
	if _, err := db.MigrateUp(ctx, p.Writer); err != nil {
		t.Fatal(err)
	}
	s := &Store{DB: p.Writer}
	it := Item{ID: "1", ClientID: "c1", Title: "buy milk"}
	if err := s.Upsert(ctx, it); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(ctx, "", "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list len=%d want 1", len(list))
	}
}
