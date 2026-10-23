package deploy

import (
	"testing"

	"github.com/irgipaulius/cogs/pkg/appcfg"
)

func TestSupportsMigrateSkipsCombined(t *testing.T) {
	if SupportsMigrate("", "/bin/false", appcfg.App{Kind: "combined"}) {
		t.Fatal("combined app must not run migrate")
	}
}
