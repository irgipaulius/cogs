package cli

import (
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
