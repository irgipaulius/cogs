package gitrepo

import "testing"

func TestFormatOwnershipWarningUsesServiceUser(t *testing.T) {
	t.Setenv("COGS_SERVICE_USER", "cogs")
	msg := FormatOwnershipWarning("/srv/cogs", []string{"/srv/cogs/.git/objects/ab"})
	if msg == "" || !contains(msg, "cogs") {
		t.Fatalf("expected cogs in warning: %q", msg)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
