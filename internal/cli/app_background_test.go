package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/irgipaulius/cogs/pkg/state"
)

func TestRecordAppBackgroundJobErrorsScraper(t *testing.T) {
	root := t.TempDir()
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	dir := filepath.Join(root, "apps", "goldilocks")
	run := filepath.Join(dir, ".cogs", "run")
	_ = os.MkdirAll(run, 0o755)
	_ = os.WriteFile(filepath.Join(run, "app.log"), []byte("scraper: can't open scripts/scripts/scrape_chunk.py\n"), 0o644)
	_ = os.WriteFile(filepath.Join(run, "app.log.start"), []byte("0\n"), 0o644)
	if err := st.SetDesired(state.AppDesired{ID: "goldilocks", Present: true, AppPath: "apps/goldilocks"}); err != nil {
		t.Fatal(err)
	}
	recordAppBackgroundJobErrors(st, "goldilocks", dir)
	v, err := st.GetApp("goldilocks")
	if err != nil {
		t.Fatal(err)
	}
	if v.LastError == "" {
		t.Fatal("expected last_error for scraper failure")
	}
}

func TestRecordAppBackgroundJobExitStatus(t *testing.T) {
	root := t.TempDir()
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	dir := filepath.Join(root, "apps", "goldilocks")
	run := filepath.Join(dir, ".cogs", "run")
	_ = os.MkdirAll(run, 0o755)
	_ = os.WriteFile(filepath.Join(run, "app.log"), []byte("scraper: exit status 1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(run, "app.log.start"), []byte("0\n"), 0o644)
	if err := st.SetDesired(state.AppDesired{ID: "goldilocks", Present: true, AppPath: "apps/goldilocks"}); err != nil {
		t.Fatal(err)
	}
	recordAppBackgroundJobErrors(st, "goldilocks", dir)
	v, err := st.GetApp("goldilocks")
	if err != nil {
		t.Fatal(err)
	}
	if v.LastError == "" {
		t.Fatal("expected last_error for exit status 1")
	}
}
