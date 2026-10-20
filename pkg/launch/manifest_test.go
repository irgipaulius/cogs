package launch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, DefaultFilename), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.APISubdomain != "todo-api" || m.Port != 0 {
		t.Fatalf("%+v", m)
	}
}
