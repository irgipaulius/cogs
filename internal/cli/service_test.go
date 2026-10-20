package cli

import "testing"

func TestServiceRenderOutputPath(t *testing.T) {
	if got := serviceRenderOutputPath("rcd", "template/deploy/rc.d/cogs"); got != "cogs-app.rc" {
		t.Fatalf("rcd out=%q", got)
	}
	if got := serviceRenderOutputPath("rcd", "template/deploy/rc.d/cogs"); got == "cogs" {
		t.Fatal("must not write repo launcher name cogs")
	}
}
