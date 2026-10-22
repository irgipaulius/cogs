package boot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteUpEnvFiltersKeys(t *testing.T) {
	dir := t.TempDir()
	if err := WriteUpEnv(dir, map[string]string{
		"COGS_DEPLOY_QUEUE_ID": "q-123",
		"COGS_LISTEN":          "127.0.0.1:9999",
	}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(UpEnvPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, "COGS_DEPLOY_QUEUE_ID=q-123") {
		t.Fatalf("missing queue id: %s", body)
	}
	if strings.Contains(body, "COGS_LISTEN") {
		t.Fatalf("COGS_LISTEN must not be stored: %s", body)
	}
}

func TestWriteUpEnvStableKeyOrder(t *testing.T) {
	dir := t.TempDir()
	if err := WriteUpEnv(dir, map[string]string{
		"COGS_CF_QUEUE_ID":     "z",
		"COGS_DEPLOY_QUEUE_ID": "a",
	}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(UpEnvPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	idxCF := strings.Index(body, "COGS_CF_QUEUE_ID")
	idxDeploy := strings.Index(body, "COGS_DEPLOY_QUEUE_ID")
	if idxCF < 0 || idxDeploy < 0 || idxCF > idxDeploy {
		t.Fatalf("expected sorted keys (CF before DEPLOY): %s", body)
	}
}

func TestLoadUpEnvIntoProcessWhitelist(t *testing.T) {
	dir := t.TempDir()
	path := UpEnvPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("COGS_DEPLOY_QUEUE_ID=from-file\nCOGS_LISTEN=bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COGS_LISTEN", "")
	t.Setenv("COGS_DEPLOY_QUEUE_ID", "")
	LoadUpEnvIntoProcess(dir)
	if got := os.Getenv("COGS_DEPLOY_QUEUE_ID"); got != "from-file" {
		t.Fatalf("queue id=%q", got)
	}
	if got := os.Getenv("COGS_LISTEN"); got != "" {
		t.Fatalf("COGS_LISTEN must not load from cogs-up.env, got %q", got)
	}
}
