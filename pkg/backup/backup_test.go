package backup

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/state"
	_ "modernc.org/sqlite"
)

func TestSQLiteRoundTrip(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "demo")
	_ = os.MkdirAll(appDir, 0o755)
	dbRel := "data.db"
	dbAbs := filepath.Join(appDir, dbRel)
	_ = os.MkdirAll(filepath.Dir(dbAbs), 0o755)
	db, err := sql.Open("sqlite", dbAbs)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(`CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT)`)
	_, _ = db.Exec(`INSERT INTO kv VALUES ('a','1')`)
	_ = db.Close()

	toml := `name = "demo"
backup = true
backup_every = "24h"
backup_dir = ".cogs/backups"
backup_db = "data.db"
`
	if err := os.WriteFile(filepath.Join(appDir, appcfg.Filename), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.SetDesired(state.AppDesired{ID: "demo", Present: true, AppPath: "apps/demo"})

	svc := &Service{WSRoot: root, Store: st}
	id, err := svc.Snapshot(ctx, "demo", appDir)
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Fatal("expected backup id")
	}
	db2, _ := sql.Open("sqlite", dbAbs)
	_, _ = db2.Exec(`UPDATE kv SET v='2' WHERE k='a'`)
	_ = db2.Close()

	row, _ := st.GetBackup("demo", id)
	stop := func() error { return nil }
	start := func() error { return nil }
	if err := svc.Restore(ctx, "demo", appDir, id, stop, start); err != nil {
		t.Fatal(err)
	}
	db3, _ := sql.Open("sqlite", dbAbs)
	var v string
	_ = db3.QueryRow(`SELECT v FROM kv WHERE k='a'`).Scan(&v)
	_ = db3.Close()
	if v != "1" {
		t.Fatalf("restore failed, v=%q", v)
	}
	_ = row
}

func TestPathEscapeRejected(t *testing.T) {
	app := t.TempDir()
	if _, err := ResolveInsideApp(app, "../../etc/passwd"); err == nil {
		t.Fatal("expected escape error")
	}
}

func TestDueSchedule(t *testing.T) {
	now := time.Now()
	if !Due(time.Time{}, time.Hour, now) {
		t.Fatal("first backup should be due")
	}
	if Due(now.Add(-10*time.Minute), time.Hour, now) {
		t.Fatal("should not be due yet")
	}
	if !Due(now.Add(-2*time.Hour), time.Hour, now) {
		t.Fatal("should be due")
	}
}

func TestCommandDriverRejectsShell(t *testing.T) {
	app := t.TempDir()
	err := RunCommandDriver(context.Background(), app, []string{"sh", "-c", "echo"}, nil)
	if err == nil {
		t.Fatal("expected rejection")
	}
}

func TestCommandDriverCopiesInsideApp(t *testing.T) {
	app := t.TempDir()
	mainGo := `package main
import ("io"; "os")
func main() {
  dest := os.Getenv("COGS_BACKUP_DEST")
  f, err := os.Open("data.db")
  if err != nil { os.Exit(1) }
  defer f.Close()
  out, err := os.Create(dest)
  if err != nil { os.Exit(1) }
  defer out.Close()
  _, _ = io.Copy(out, f)
}
`
	if err := os.WriteFile(filepath.Join(app, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(app, "go.mod"), []byte("module snaptest\n\ngo 1.22\n"), 0o644)
	bin := filepath.Join(app, "snapbin")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = app
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build tool: %v: %s", err, out)
	}
	_ = os.Chmod(bin, 0o755)
	if err := os.WriteFile(filepath.Join(app, "data.db"), []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(app, "out.db")
	if err := RunCommandDriver(context.Background(), app, []string{"./snapbin"}, map[string]string{
		"COGS_APP_DIR":     app,
		"COGS_BACKUP_DEST": dest,
	}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dest)
	if err != nil || string(b) != "payload" {
		t.Fatal("expected copied payload")
	}
}

func TestRetentionPrunesOldBackups(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "demo")
	_ = os.MkdirAll(appDir, 0o755)
	dbAbs := filepath.Join(appDir, "data.db")
	db, _ := sql.Open("sqlite", dbAbs)
	_, _ = db.Exec(`CREATE TABLE t(x)`)
	_ = db.Close()
	toml := `name = "demo"
backup = true
backup_keep = 2
backup_db = "data.db"
`
	_ = os.WriteFile(filepath.Join(appDir, appcfg.Filename), []byte(toml), 0o644)
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	_ = st.SetDesired(state.AppDesired{ID: "demo", Present: true, AppPath: "apps/demo"})
	svc := &Service{WSRoot: root, Store: st}
	for i := 0; i < 4; i++ {
		if _, err := svc.Snapshot(ctx, "demo", appDir); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	rows, err := st.ListBackups("demo", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) > 2 {
		t.Fatalf("expected at most 2 backups, got %d", len(rows))
	}
}
