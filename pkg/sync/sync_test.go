package sync

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/irgipaulius/cogs/pkg/sqliteopen"
)

type fakeD1 struct {
	batch []Statement
}

func (f *fakeD1) Query(ctx context.Context, sql string, params ...any) ([][]string, error) {
	return nil, nil
}

func (f *fakeD1) ExecBatch(ctx context.Context, statements []Statement) error {
	f.batch = append(f.batch, statements...)
	return nil
}

type fakeBlobs struct {
	objects map[string]string
	puts    int
}

func (f *fakeBlobs) HeadHash(ctx context.Context, key string) (string, bool, error) {
	h, ok := f.objects[key]
	return h, ok, nil
}

func (f *fakeBlobs) PutBlob(ctx context.Context, key string, data []byte) (string, error) {
	h := RowHash(string(data))
	f.objects[key] = h
	f.puts++
	return h, nil
}

func TestOutboxOneUpsertAmongMany(t *testing.T) {
	dir := t.TempDir()
	pools, err := sqliteopen.Open(sqliteopen.Config{Path: filepath.Join(dir, "t.db"), ReaderConns: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer pools.Close()
	db := pools.Writer
	ctx := context.Background()
	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY, val TEXT, blob BLOB)`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		_, err = db.Exec(`INSERT INTO items (id, val, blob) VALUES (?, ?, ?)`,
			fmtID(i), "v", []byte("b"))
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := EnqueueOutbox(ctx, db, "items", "id-1", "upsert", RowHash("id-1", "v"), RowHash("b")); err != nil {
		t.Fatal(err)
	}
	d1 := &fakeD1{}
	blobs := &fakeBlobs{objects: map[string]string{}}
	eng := Engine{
		DB: db, D1: d1, Blobs: blobs,
		SetName: "items",
		Config: Config{Sets: []Projection{{
			Name: "items", Table: "items", PrimaryKey: "id",
			Columns: []string{"val"}, BlobColumn: "blob", D1Table: "items_edge",
		}}},
	}
	st, err := eng.PushOutbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Dirty != 1 {
		t.Fatalf("dirty=%d want 1", st.Dirty)
	}
	if len(d1.batch) != 1 {
		t.Fatalf("batch len=%d want 1", len(d1.batch))
	}
}

func TestBlobSkipWhenHashExists(t *testing.T) {
	blobs := &fakeBlobs{objects: map[string]string{
		blobKey("s", "abc"): "abc",
	}}
	ctx := context.Background()
	sk, err := (&Engine{Blobs: blobs, SetName: "s"}).blobSkipped(ctx, Projection{}, "abc")
	if err != nil || !sk {
		t.Fatalf("skip=%v err=%v", sk, err)
	}
}

func TestChecksumOneDirtyBucket(t *testing.T) {
	dir := t.TempDir()
	pools, err := sqliteopen.Open(sqliteopen.Config{Path: filepath.Join(dir, "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer pools.Close()
	db := pools.Writer
	ctx := context.Background()
	_ = EnsureSchema(ctx, db)
	_, _ = db.Exec(`CREATE TABLE t (id TEXT PRIMARY KEY)`)
	_, _ = db.Exec(`INSERT INTO t (id) VALUES ('a'), ('b')`)
	d1 := &fakeD1{}
	eng := Engine{
		DB: db, D1: d1, SetName: "t",
		Config: Config{Sets: []Projection{{
			Name: "t", Table: "t", PrimaryKey: "id", Columns: []string{}, BlockBuckets: 512,
		}}},
	}
	st, err := eng.ChecksumSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Dirty < 0 {
		t.Fatal("bad stats")
	}
}

func fmtID(i int) string {
	return fmt.Sprintf("id-%d", i)
}

// silence unused import if sql not used in fmtID path
var _ = sql.ErrNoRows
