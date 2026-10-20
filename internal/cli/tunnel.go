package cli

import (
	"flag"
	"fmt"
	"path/filepath"
)

func cmdTunnel(args []string) error {
	if len(args) < 1 {
		return tunnelUsage()
	}
	switch args[0] {
	case "init":
		return tunnelInit(args[1:])
	case "private":
		return tunnelPrivate(args[1:])
	case "run":
		return tunnelRunCmd(args[1:])
	case "start":
		return tunnelStartCmd(args[1:])
	case "stop":
		return tunnelStopCmd(args[1:])
	case "status":
		return tunnelStatusCmd(args[1:])
	case "fix":
		return tunnelFixCmd(args[1:])
	default:
		return fmt.Errorf("unknown tunnel subcommand %q", args[0])
	}
}

func tunnelUsage() error {
	fmt.Print(`Usage:
  cogs tunnel run <name>              foreground (uses npx wrangler if needed)
  cogs tunnel start [name] [--dir .]  background pid + log under .cogs/run/
  cogs tunnel stop [name] [--dir .]
  cogs tunnel status [name] [--dir .]
  cogs tunnel fix [--dir .]     repair ingress + DNS for API hostname
  cogs tunnel init <hostname>
  cogs tunnel private [--name NAME] [--port PORT]

`)
	return nil
}

func tunnelRunCmd(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	_ = fs.Parse(args)
	name := fs.Arg(0)
	if name == "" {
		name, _ = tunnelNameFromProject(".")
	}
	return tunnelRunForeground(name)
}

func tunnelStartCmd(args []string) error {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	dir := fs.String("dir", ".", "project directory")
	_ = fs.Parse(args)
	name := fs.Arg(0)
	abs, err := absDir(*dir)
	if err != nil {
		return err
	}
	return startTunnelBackground(abs, name)
}

func tunnelStopCmd(args []string) error {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	dir := fs.String("dir", ".", "project directory")
	_ = fs.Parse(args)
	abs, err := absDir(*dir)
	if err != nil {
		return err
	}
	return stopTunnel(abs, fs.Arg(0))
}

func tunnelFixCmd(args []string) error {
	fs := flag.NewFlagSet("fix", flag.ExitOnError)
	dir := fs.String("dir", ".", "project directory")
	_ = fs.Parse(args)
	abs, err := absDir(*dir)
	if err != nil {
		return err
	}
	return tunnelFix(abs)
}

func tunnelStatusCmd(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	dir := fs.String("dir", ".", "project directory")
	_ = fs.Parse(args)
	abs, err := absDir(*dir)
	if err != nil {
		return err
	}
	return tunnelStatus(abs, fs.Arg(0))
}

func tunnelInit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cogs tunnel init <hostname>")
	}
	host := args[0]
	return wrangler("tunnel", "create", host)
}

func tunnelPrivate(args []string) error {
	fs := flag.NewFlagSet("private", flag.ExitOnError)
	name := fs.String("name", "cogs-private", "tunnel name")
	port := fs.Int("port", 8080, "local HTTP port")
	_ = fs.Parse(args)

	if err := wrangler("tunnel", "create", *name); err != nil {
		return err
	}
	fmt.Println("Start the tunnel (no global wrangler required):")
	fmt.Printf("  cogs tunnel start %s\n", *name)
	fmt.Println("Then create a VPC service:")
	fmt.Printf("  npx wrangler vpc service create cogs-api --type http --tunnel-id <TUNNEL_ID> --ipv4 127.0.0.1 --http-port %d\n", *port)
	fmt.Println("Set COGS_PRIVATE_TOKEN in your app env and send X-Cogs-Private-Token from the Worker.")
	return nil
}

func absDir(dir string) (string, error) {
	return filepath.Abs(dir)
}
