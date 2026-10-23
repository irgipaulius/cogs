package cli

import (
	"fmt"
	"strings"

	"github.com/irgipaulius/cogs/pkg/version"
)

// Run dispatches cogs subcommands.
func Run(args []string) error {
	args = peelGlobalFlags(args)
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "new":
		return cmdNew(args[1:])
	case "build":
		return cmdBuild(args[1:])
	case "deploy":
		return cmdDeploy(args[1:])
	case "ping":
		return cmdPing(args[1:])
	case "launch":
		return cmdLaunch(args[1:])
	case "redeploy":
		return cmdLaunch(append([]string{"--quick", "-y"}, args[1:]...))
	case "service":
		return cmdService(args[1:])
	case "tunnel":
		return cmdTunnel(args[1:])
	case "sync":
		if len(args) >= 2 && args[1] == "schema" {
			return cmdSyncSchema(args[2:])
		}
		return cmdSync(args[1:])
	case "deps":
		return cmdDeps(args[1:])
	case "restore-test":
		return cmdRestoreTest(args[1:])
	case "doctor":
		return cmdDoctor(args[1:])
	case "up":
		return cmdUp(args[1:])
	case "down":
		return cmdDown(args[1:])
	case "remove":
		return cmdRemove(args[1:])
	case "github":
		return cmdGitHub(args[1:])
	case "host":
		return cmdHost(args[1:])
	case "cf":
		return cmdCF(args[1:])
	case "admin":
		return cmdAdmin(args[1:])
	case "supervisor":
		return cmdSupervisor(args[1:])
	case "apps":
		return cmdApps(args[1:])
	case "workspace":
		if len(args) >= 2 && args[1] == "apps" {
			return cmdApps(args[2:])
		}
		return cmdApps(args[1:])
	case "version":
		fmt.Println("cogs", version.String())
		return nil
	case "help", "-h", "--help":
		return usage()
	default:
		if isLaunchProjectPath(args[0]) {
			return cmdLaunch(args)
		}
		return fmt.Errorf("unknown command %q (or path to a project with cogs.launch.json)", args[0])
	}
}

func usage() error {
	fmt.Print(`CoGS — Cloudflare app orchestrator (Go + SQLite on your hardware)

Usage:
  cogs apps list                      apps from cogs.workspace.json
  cogs new <project> [--module path] [--with geo,jobs]
  cogs build [--target os/arch,...]
  cogs launch [app|path] [-y] [--quick]   provision or fast redeploy
  cogs redeploy [app|path]            launch --quick -y
  cogs github login|status|logout     host gh auth (or PAT via --token)
  cogs <app|path>                     launch when path or workspace app name resolves
  cogs deploy [--app NAME]... [--ping] [--dry-run]
              deploy all apps on this host
  cogs deploy --remote <user@host> [--target os/arch]
  cogs ping [app|path] [--link] [--url URL]
  Global: --json --dry-run -q --verbose --no-color
  cogs service render <systemd|rcd|launchd>
  cogs tunnel run|start|stop|status [name] [--dir .]
  cogs tunnel init <hostname>
  cogs tunnel private [--name NAME] [--port PORT]
  cogs sync [--delete] | cogs sync --checksum | cogs sync --rebuild
  cogs deps --target os/arch
  cogs restore-test <backup.db>
  cogs admin init|check           public dashboard bootstrap
  cogs supervisor run             control plane (unix socket)
  cogs doctor                     check zone, DNS token, tunnels, ports, go.work, config.js
  cogs up [--foreground]          supervise apps and tunnels; writes .cogs/boot/cogs-up.sh
  cogs down                       stop the apps and tunnels cogs up started
  cogs remove <app>               remove app, Cloudflare resources, and local artifacts
  cogs cf whoami | r2 | d1 | pages | worker | kv ...
  cogs version

`)
	return nil
}

func splitTargets(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
