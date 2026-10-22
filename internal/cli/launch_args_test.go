package cli

import "testing"

func TestReorderLaunchArgs(t *testing.T) {
	got := reorderLaunchArgs([]string{"admin", "-y", "--skip-pages"})
	want := []string{"-y", "--skip-pages", "admin"}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d]=%q want %q full=%v", i, got[i], want[i], got)
		}
	}
	f, err := parseLaunchFlags([]string{"admin", "-y"})
	if err != nil {
		t.Fatal(err)
	}
	if !f.yes {
		t.Fatal("expected yes from trailing -y")
	}
	if f.dir != "admin" {
		t.Fatalf("dir=%q want admin", f.dir)
	}
}
