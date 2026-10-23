package scraper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveScriptPathFromAppDir(t *testing.T) {
	app := t.TempDir()
	scriptDir := filepath.Join(app, "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(scriptDir, "scrape_chunk.py")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env python3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COGS_WORKDIR", scriptDir)
	got := ResolveScriptPath(app, "scripts/scrape_chunk.py")
	if got != script {
		t.Fatalf("path=%q want %q", got, script)
	}
}
