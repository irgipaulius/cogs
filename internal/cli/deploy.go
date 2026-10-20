package cli

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func cmdDeploy(args []string) error {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	target := fs.String("target", "", "GOOS/GOARCH for cross-build (remote SSH deploy)")
	service := fs.String("service", "systemd", "service kind")
	rollback := fs.Bool("rollback", false, "roll back to .prev binary")
	remote := fs.Bool("remote", false, "force SSH deploy (requires user@host)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()

	if len(rest) >= 1 && (strings.Contains(rest[0], "@") || *remote) {
		return cmdDeployRemote(rest, deployRemoteOpts{
			target:   *target,
			service:  *service,
			rollback: *rollback,
		})
	}
	return cmdDeployWorkspace(rest)
}

type deployRemoteOpts struct {
	target   string
	service  string
	rollback bool
}

func cmdDeployRemote(rest []string, o deployRemoteOpts) error {
	if len(rest) < 1 {
		return fmt.Errorf("usage: cogs deploy --remote <user@host>")
	}
	host := rest[0]
	if o.rollback {
		return remoteRollback(host)
	}
	if err := cmdBuild([]string{"--target", defaultTarget(o.target)}); err != nil {
		return err
	}
	bin := localBinary(o.target)
	remotePath := "/usr/local/bin/cogs-app"
	scp := exec.Command("scp", bin, host+":"+remotePath+".new")
	scp.Stdout = os.Stdout
	scp.Stderr = os.Stderr
	if err := scp.Run(); err != nil {
		return err
	}
	script := fmt.Sprintf(`
set -e
if [ -f %[1]s ]; then mv %[1]s %[1]s.prev; fi
mv %[1]s.new %[1]s
chmod +x %[1]s
sudo systemctl restart cogs-app || true
`, remotePath)
	ssh := exec.Command("ssh", host, script)
	ssh.Stdout = os.Stdout
	ssh.Stderr = os.Stderr
	if err := ssh.Run(); err != nil {
		_ = remoteRollback(host)
		return err
	}
	healthURL := "https://" + strings.Split(host, "@")[1] + "/health"
	if err := pollHealth(healthURL); err != nil {
		_ = remoteRollback(host)
		return err
	}
	fmt.Println("deploy ok:", o.service)
	return nil
}

func defaultTarget(t string) string {
	if t != "" {
		return t
	}
	return "linux/amd64"
}

func localBinary(target string) string {
	tg := defaultTarget(target)
	parts := strings.Split(tg, "/")
	return fmt.Sprintf("dist/%s_%s/app", parts[0], parts[1])
}

func remoteRollback(host string) error {
	script := `set -e
if [ -f /usr/local/bin/cogs-app.prev ]; then
  mv /usr/local/bin/cogs-app.prev /usr/local/bin/cogs-app
  sudo systemctl restart cogs-app || true
fi`
	ssh := exec.Command("ssh", host, script)
	ssh.Stdout = os.Stdout
	ssh.Stderr = os.Stderr
	return ssh.Run()
}

func pollHealth(url string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	for i := 0; i < 12; i++ {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("health check failed for %s", url)
}
