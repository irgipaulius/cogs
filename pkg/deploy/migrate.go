package deploy

import (
	"os/exec"
	"strings"

	"github.com/irgipaulius/cogs/pkg/appcfg"
)

// SupportsMigrate reports whether the release binary implements `migrate`.
func SupportsMigrate(appDir, binPath string, cfg appcfg.App) bool {
	if cfg.Kind == "combined" {
		return false
	}
	out, err := exec.Command(binPath).CombinedOutput()
	if err == nil {
		return true
	}
	s := strings.ToLower(string(out))
	return strings.Contains(s, "migrate")
}
