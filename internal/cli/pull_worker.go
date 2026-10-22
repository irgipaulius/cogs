package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func deployPullWorkerUI(projectDir, pageHost, apiHost string) error {
	pageHost = strings.TrimSpace(pageHost)
	apiHost = strings.TrimSpace(apiHost)
	if pageHost == "" || apiHost == "" {
		return fmt.Errorf("page and api hostnames required for Worker deploy")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found — install Node.js for pull mode UI")
	}
	apiURL := "https://" + strings.TrimPrefix(apiHost, "https://")
	pageURL := "https://" + strings.TrimPrefix(pageHost, "https://")
	env := append(os.Environ(),
		"COGS_PUBLIC_API_URL="+apiURL,
		"COGS_PUBLIC_PAGE_ORIGIN="+pageURL,
	)
	if err := npmWithEnv(projectDir, env, "run", "build"); err != nil {
		return fmt.Errorf("npm run build: %w", err)
	}
	cmd, err := wranglerCmd("deploy")
	if err != nil {
		return err
	}
	cmd.Dir = projectDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wrangler deploy: %w", err)
	}
	fmt.Printf("Pull mode: Worker UI deployed at https://%s\n", pageHost)
	return nil
}

func npmWithEnv(dir string, env []string, args ...string) error {
	cmd := exec.Command("npm", args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func pullBuildCommand(apiHost, pageHost string) string {
	apiURL := "https://" + strings.TrimPrefix(strings.TrimSpace(apiHost), "https://")
	pageURL := "https://" + strings.TrimPrefix(strings.TrimSpace(pageHost), "https://")
	return fmt.Sprintf("COGS_PUBLIC_API_URL=%q COGS_PUBLIC_PAGE_ORIGIN=%q npm run build", apiURL, pageURL)
}
