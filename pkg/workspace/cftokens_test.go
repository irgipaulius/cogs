package workspace

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoadDNSToken(t *testing.T) {
	dir := t.TempDir()
	if err := SaveDNSToken(dir, "example.com", "secret-token"); err != nil {
		t.Fatal(err)
	}
	got, err := DNSTokenForZone(dir, "example.com")
	if err != nil || got != "secret-token" {
		t.Fatalf("zone token: %q err=%v", got, err)
	}
	got, err = DNSTokenForZone(dir, "other.com")
	if err != nil || got != "secret-token" {
		t.Fatalf("default token fallback: %q err=%v", got, err)
	}
	reg, err := LoadCFTokensRegistry(dir)
	if err != nil || len(reg.Entries) != 1 {
		t.Fatalf("registry: %+v err=%v", reg, err)
	}
	if reg.Entries[0].File != filepath.Base(zoneTokenFile("example.com")) {
		t.Fatalf("entry file: %s", reg.Entries[0].File)
	}
}
