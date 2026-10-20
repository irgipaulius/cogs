package jobs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/irgipaulius/cogs/pkg/sqliteopen"
)

func TestOverlapLock(t *testing.T) {
	p, err := sqliteopen.Open(sqliteopen.Config{Path: filepath.Join(t.TempDir(), "j.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()
	if err := EnsureSchema(ctx, p.Writer); err != nil {
		t.Fatal(err)
	}
	ok1, err := TryLock(ctx, p.Writer, "scrape", time.Minute)
	if err != nil || !ok1 {
		t.Fatalf("lock1 ok=%v err=%v", ok1, err)
	}
	ok2, err := TryLock(ctx, p.Writer, "scrape", time.Minute)
	if err != nil || ok2 {
		t.Fatalf("lock2 ok=%v err=%v", ok2, err)
	}
}
