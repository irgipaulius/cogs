package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPullBuildCommandSetsAPIURL(t *testing.T) {
	cmd := pullBuildCommand("goldilocks-api.hyperreader.eu", "goldilocks.hyperreader.eu")
	if !strings.Contains(cmd, "COGS_PUBLIC_API_URL=\"https://goldilocks-api.hyperreader.eu\"") {
		t.Fatalf("build cmd missing api url: %s", cmd)
	}
	if !strings.Contains(cmd, "COGS_PUBLIC_PAGE_ORIGIN") {
		t.Fatalf("build cmd missing page origin: %s", cmd)
	}
}

func TestWritePullWorkerWranglerCustomDomain(t *testing.T) {
	dir := t.TempDir()
	if err := writePullWorkerWrangler(dir, "cogs-goldilocks", "goldilocks.hyperreader.eu"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "wrangler.toml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if !strings.Contains(body, `pattern = "goldilocks.hyperreader.eu"`) {
		t.Fatalf("missing route: %s", body)
	}
	if !strings.Contains(body, "custom_domain = true") {
		t.Fatalf("missing custom_domain: %s", body)
	}
}
