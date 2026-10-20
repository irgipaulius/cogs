package cli

import (
	"fmt"
	"strings"
)

// Run dispatches cogs subcommands.
func Run(args []string) error {
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
	case "cf":
		return cmdCF(args[1:])
	case "apps":
		return cmdApps(args[1:])
	case "workspace":
		if len(args) >= 2 && args[1] == "apps" {
			return cmdApps(args[2:])
		}
		return cmdApps(args[1:])
	case "version":
		fmt.Println("cogs dev")
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
  cogs <app|path>                     launch when path or workspace app name resolves
  cogs deploy <ssh-host> [--target os/arch] [--service systemd|rcd|launchd]
  cogs service render <systemd|rcd|launchd>
  cogs tunnel run|start|stop|status [name] [--dir .]
  cogs tunnel init <hostname>
  cogs tunnel private [--name NAME] [--port PORT]
  cogs sync [--delete] | cogs sync --checksum | cogs sync --rebuild
  cogs deps --target os/arch
  cogs restore-test <backup.db>
  cogs doctor [-env .env]
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
