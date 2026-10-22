package hostsetup

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// EnsureGitHubCLI installs gh when missing (best effort).
func EnsureGitHubCLI() error {
	if _, err := exec.LookPath("gh"); err == nil {
		return nil
	}
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err != nil {
			return fmt.Errorf("install GitHub CLI: brew install gh")
		}
		cmd := exec.Command("brew", "install", "gh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "freebsd":
		cmd := exec.Command("pkg", "install", "-y", "gh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "linux":
		if _, err := exec.LookPath("apt-get"); err == nil {
			_ = exec.Command("sudo", "apt-get", "update").Run()
			cmd := exec.Command("sudo", "apt-get", "install", "-y", "gh")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		return fmt.Errorf("install GitHub CLI: see https://cli.github.com/")
	default:
		return fmt.Errorf("install GitHub CLI: see https://cli.github.com/")
	}
}

// GitHubLoginHint returns shell commands for the service user (respects COGS_HOME).
func GitHubLoginHint() []string {
	home := strings.TrimSpace(os.Getenv("COGS_HOME"))
	prefix := ""
	if home != "" {
		prefix = fmt.Sprintf("HOME=%s ", home)
	}
	return []string{
		prefix + "gh auth login --hostname github.com --git-protocol https",
		prefix + "gh auth setup-git",
	}
}
