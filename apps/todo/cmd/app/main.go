package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/irgipaulius/cogs/apps/todo/internal/db"
	"github.com/irgipaulius/cogs/apps/todo/internal/todo"
	"github.com/irgipaulius/cogs/pkg/config"
	"github.com/irgipaulius/cogs/pkg/cors"
	"github.com/irgipaulius/cogs/pkg/limiter"
	"github.com/irgipaulius/cogs/pkg/observe"
	"github.com/irgipaulius/cogs/pkg/r2"
	"github.com/irgipaulius/cogs/pkg/server"
	"github.com/irgipaulius/cogs/pkg/session"
	"github.com/irgipaulius/cogs/pkg/sqliteopen"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	if cmd == "version" {
		fmt.Println("todo-app 0.1.0")
		return
	}
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	envFile := fs.String("env", ".env", "env file path (optional overlay; COGS_* env vars always apply)")
	_ = fs.Parse(os.Args[2:])

	ctx := context.Background()
	switch cmd {
	case "serve":
		if err := serve(ctx, *envFile); err != nil {
			log.Fatal(err)
		}
	case "migrate":
		if err := migrate(ctx, *envFile); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: app serve|migrate|version [-env .env]")
}

func openDB(envFile string) (*sqliteopen.Pools, config.App, error) {
	cfg, err := config.Load(envFile)
	if err != nil {
		return nil, config.App{}, err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o750); err != nil {
		return nil, config.App{}, err
	}
	pools, err := sqliteopen.Open(sqliteopen.Config{Path: cfg.DBPath, ReaderConns: 8})
	return pools, cfg, err
}

func migrate(ctx context.Context, envFile string) error {
	pools, _, err := openDB(envFile)
	if err != nil {
		return err
	}
	defer pools.Close()
	v, err := db.MigrateUp(ctx, pools.Writer)
	if err != nil {
		return err
	}
	fmt.Println("migration version:", v)
	return nil
}

func serve(ctx context.Context, envFile string) error {
	pools, cfg, err := openDB(envFile)
	if err != nil {
		return err
	}
	defer pools.Close()
	ver, err := db.MigrateUp(ctx, pools.Writer)
	if err != nil {
		return err
	}

	obs, err := observe.Setup(cfg)
	if err != nil {
		return err
	}
	defer obs.Close()
	if obs.Logger != nil {
		slog.SetDefault(obs.Logger)
	}

	var r2c *r2.Client
	if cfg.R2AccountID != "" && cfg.R2AccessKeyID != "" {
		r2c, err = r2.New(ctx, r2.Config{
			AccountID: cfg.R2AccountID, AccessKeyID: cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey, Bucket: cfg.R2Bucket,
		})
		if err != nil {
			log.Printf("r2 disabled: %v", err)
			r2c = nil
		}
	}

	dbWriter := observe.InstrumentDB(pools.Writer, obs.SQL)
	api := &todo.API{
		Store:     &todo.Store{DB: dbWriter},
		R2:        r2c,
		MaxUpload: cfg.MaxUploadBytes,
		R2Prefix:  "todo-images",
	}

	origins := []string{"http://127.0.0.1:8788", "http://localhost:8788"}
	if cfg.PublicPageOrigin != "" {
		origins = append(origins, cfg.PublicPageOrigin)
	}

	mux := server.NewMux(server.Deps{
		Config:       cfg,
		DB:           pools.Reader,
		Session:      &session.Manager{Secret: []byte(cfg.SessionSecret), TurnstileSecret: cfg.TurnstileSecret},
		IPLimiter:    limiter.NewLRU(20_000, 30, 60),
		MigrationVer: ver,
		Observe:      obs,
		RegisterRoutes: func(m *http.ServeMux) {
			api.Register(m)
		},
	})
	handler := cors.Middleware(origins)(mux)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		slog.Info("listening", "addr", cfg.ListenAddr)
		if obs.AdminEnabled() {
			slog.Info("local admin", "url", "http://"+cfg.ListenAddr+"/admin (127.0.0.1 / localhost Host only)")
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-sig
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}
