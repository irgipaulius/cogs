package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/irgipaulius/cogs/apps/admin/internal/api"
	"github.com/irgipaulius/cogs/apps/admin/internal/ui"
	"github.com/irgipaulius/cogs/pkg/adminstore"
	"github.com/irgipaulius/cogs/pkg/config"
	"github.com/irgipaulius/cogs/pkg/supervisor"
	"github.com/irgipaulius/cogs/pkg/workspace"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	envFile := fs.String("env", ".env", "env file")
	_ = fs.Parse(os.Args[2:])
	switch os.Args[1] {
	case "serve":
		if err := serve(*envFile); err != nil {
			log.Fatal(err)
		}
	case "version":
		fmt.Println("cogs-admin 0.1.0")
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: cogs-admin serve|version [-env .env]")
}

func serve(envFile string) error {
	cfg, err := config.LoadRelaxed(envFile)
	if err != nil {
		return err
	}
	wsRoot := cfg.WorkspaceRoot
	if wsRoot == "" {
		wd, _ := os.Getwd()
		wsRoot, _, _ = workspace.FindRoot(wd)
	}
	adminDB := cfg.AdminDBPath
	if adminDB == "" {
		adminDB = "data/admin.db"
	}
	if !filepath.IsAbs(adminDB) {
		baseDir, _ := os.Getwd()
		if envFile != "" {
			p := envFile
			if !filepath.IsAbs(p) {
				p = filepath.Join(baseDir, p)
			}
			baseDir = filepath.Dir(p)
		}
		adminDB = filepath.Join(baseDir, adminDB)
	}
	if err := os.MkdirAll(filepath.Dir(adminDB), 0o750); err != nil {
		return err
	}
	store, err := adminstore.Open(adminDB)
	if err != nil {
		return err
	}
	defer store.Close()

	sock := cfg.SupervisorSocket
	if sock == "" && wsRoot != "" {
		sock = supervisor.DefaultSocketPath(wsRoot)
	}

	mux := http.NewServeMux()
	srv := &api.Server{
		Store:        store,
		Supervisor:   &supervisor.Client{SocketPath: sock},
		SecureCookie: strings.HasPrefix(cfg.PublicAPIURL, "https://"),
	}
	srv.Handler(mux, ui.Handler())

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
	}
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	}()
	log.Printf("admin listening on %s", cfg.ListenAddr)
	return httpSrv.ListenAndServe()
}
