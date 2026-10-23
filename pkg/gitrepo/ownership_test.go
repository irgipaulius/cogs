package gitrepo

import "testing"

func TestFormatOwnershipWarning(t *testing.T) {
	msg := FormatOwnershipWarning("/srv/cogs", []string{"/srv/cogs/.git/objects/ab"})
	if msg == "" {
		t.Fatal("expected warning")
	}
}
