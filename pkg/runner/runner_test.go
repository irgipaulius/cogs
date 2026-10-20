package runner

import (
	"testing"
)

func TestEnsureIDStable(t *testing.T) {
	dir := t.TempDir()
	a, err := EnsureID(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := EnsureID(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a != b || a == "" {
		t.Fatalf("ids %q %q", a, b)
	}
}
