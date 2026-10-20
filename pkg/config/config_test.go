package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRefusePublicBind(t *testing.T) {
	vars := map[string]string{
		"COGS_LISTEN":        "0.0.0.0:8080",
		"COGS_SESSION_SECRET": "x",
	}
	_, err := parse(vars)
	if err == nil {
		t.Fatal("expected refuse public bind")
	}
}

func TestLoadEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("COGS_SESSION_SECRET=fromfile\nCOGS_LISTEN=127.0.0.1:9090\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.SessionSecret != "fromfile" || c.ListenAddr != "127.0.0.1:9090" {
		t.Fatalf("%+v", c)
	}
}
