package reconcile

import (
	"os/exec"
	"strings"
)

func gitHEAD(wsRoot string) string {
	out, err := exec.Command("git", "-C", wsRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
