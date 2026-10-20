package cli

import (
	"fmt"
)

func cmdCF(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cogs cf whoami|...")
	}
	switch args[0] {
	case "whoami":
		return wrangler("whoami")
	case "r2":
		return cfR2(args[1:])
	case "d1":
		return cfD1(args[1:])
	case "pages":
		return cfPages(args[1:])
	case "worker":
		return cfWorker(args[1:])
	case "kv":
		return cfKV(args[1:])
	default:
		return fmt.Errorf("unknown cf subcommand %q", args[0])
	}
}

func cfR2(args []string) error {
	if len(args) >= 2 && args[0] == "create" {
		return wrangler("r2", "bucket", "create", args[1])
	}
	return fmt.Errorf("usage: cogs cf r2 create <bucket>")
}

func cfD1(args []string) error {
	if len(args) >= 2 && args[0] == "create" {
		return wrangler("d1", "create", args[1])
	}
	return fmt.Errorf("usage: cogs cf d1 create <name>")
}

func cfPages(args []string) error {
	if len(args) >= 1 && args[0] == "deploy" {
		rest := args[1:]
		return wrangler(append([]string{"pages", "deploy"}, rest...)...)
	}
	return fmt.Errorf("usage: cogs cf pages deploy [dir]")
}

func cfWorker(args []string) error {
	if len(args) >= 1 && args[0] == "deploy" {
		return wrangler("deploy")
	}
	return fmt.Errorf("usage: cogs cf worker deploy")
}

func cfKV(args []string) error {
	if len(args) >= 2 && args[0] == "namespace" && args[1] == "create" {
		if len(args) < 3 {
			return fmt.Errorf("usage: cogs cf kv namespace create <name>")
		}
		return wrangler("kv", "namespace", "create", args[2])
	}
	return fmt.Errorf("usage: cogs cf kv namespace create <name>")
}
