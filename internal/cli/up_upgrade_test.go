package cli

import "testing"

func TestCommitsEqualPrefix(t *testing.T) {
	if !commitsEqual("1b3bfa4deadbeef", "1b3bfa4") {
		t.Fatal("expected prefix match for monorepo rebuild")
	}
}
