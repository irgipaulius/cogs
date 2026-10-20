//go:build ignore

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"{{.Module}}/internal/app"
)

func main() {
	envFile := flag.String("env", ".env", "path to env file")
	flag.Parse()
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	ctx := context.Background()

	switch cmd {
	case "serve":
		if err := app.Serve(ctx, *envFile); err != nil {
			log.Fatal(err)
		}
	case "job":
		if len(args) < 1 {
			log.Fatal("usage: app job <name>")
		}
		if err := app.RunJob(ctx, *envFile, args[0]); err != nil {
			log.Fatal(err)
		}
	case "migrate":
		if err := app.Migrate(ctx, *envFile); err != nil {
			log.Fatal(err)
		}
	case "backup":
		if err := app.Backup(ctx, *envFile); err != nil {
			log.Fatal(err)
		}
	case "version":
		fmt.Println(app.Version)
	case "sync":
		if err := app.Sync(ctx, *envFile, args); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
  app serve [-env .env]
  app job <name> [-env .env]
  app migrate [-env .env]
  app backup [-env .env]
  app sync [--checksum|--rebuild] [--delete] [-env .env]
  app version
`)
}
