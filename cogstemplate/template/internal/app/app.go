//go:build ignore

package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/irgipaulius/cogs/pkg/config"
	"github.com/irgipaulius/cogs/pkg/ingress"
	"github.com/irgipaulius/cogs/pkg/jobs"
	"github.com/irgipaulius/cogs/pkg/limiter"
	"github.com/irgipaulius/cogs/pkg/observe"
	"github.com/irgipaulius/cogs/pkg/server"
	"github.com/irgipaulius/cogs/pkg/session"
	"github.com/irgipaulius/cogs/pkg/sqliteopen"
	"{{.Module}}/internal/db"
)

// Version is set at link time.
var Version = "dev"

func openPools(envFile string) (*sqliteopen.Pools, config.App, error) {
	cfg, err := config.Load(envFile)
	if err != nil {
		return nil, config.App{}, err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o750); err != nil && !os.IsExist(err) {
		return nil, config.App{}, err
	}
	pools, err := sqliteopen.Open(sqliteopen.Config{Path: cfg.DBPath, ReaderConns: 8})
	if err != nil {
		return nil, config.App{}, err
	}
	return pools, cfg, nil
}

// Serve starts the HTTP API.
func Serve(ctx context.Context, envFile string) error {
	pools, cfg, err := openPools(envFile)
	if err != nil {
		return err
	}
	defer pools.Close()
	ver, err := db.MigrateUp(ctx, pools.Writer)
	if err != nil {
		return err
	}
	_ = jobs.EnsureSchema(ctx, pools.Writer)

	obs, err := observe.Setup(cfg)
	if err != nil {
		return err
	}
	defer obs.Close()
	if obs.Logger != nil {
		slog.SetDefault(obs.Logger)
	}

	sess := &session.Manager{
		Secret:          []byte(cfg.SessionSecret),
		TurnstileSecret: cfg.TurnstileSecret,
	}
	ingressCfg, err := ingress.LoadOptional(ingress.ResolvePath(envFile))
	if err != nil {
		return fmt.Errorf("ingress config: %w", err)
	}
	mux := server.NewMux(server.Deps{
		Config:       cfg,
		DB:           pools.Reader,
		Session:      sess,
		IPLimiter:    limiter.NewLRU(10_000, 20, 40),
		MigrationVer: ver,
		PrivateOnly:  cfg.PrivateToken != "",
		Observe:      obs,
		Ingress:      ingressCfg,
		RegisterRoutes: func(m *http.ServeMux) {
			registerAppRoutes(m, pools.Reader)
		},
	})
	srv := server.ListenAndServe(cfg.ListenAddr, mux)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-sig:
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
	return nil
}

func registerAppRoutes(m *http.ServeMux, db *sql.DB) {
	m.HandleFunc("GET /api/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"hello from CoGS"}`))
	})
}

// RunJob executes a registered job by name.
func RunJob(ctx context.Context, envFile, name string) error {
	pools, _, err := openPools(envFile)
	if err != nil {
		return err
	}
	defer pools.Close()
	_ = jobs.EnsureSchema(ctx, pools.Writer)
	reg := jobs.Registry{
		"example": func(ctx context.Context) error {
			fmt.Println("example job ran")
			return nil
		},
	}
	return jobs.Run(ctx, pools.Writer, reg, name, 30*time.Minute)
}

// Migrate applies embedded migrations.
func Migrate(ctx context.Context, envFile string) error {
	pools, _, err := openPools(envFile)
	if err != nil {
		return err
	}
	defer pools.Close()
	_, err = db.MigrateUp(ctx, pools.Writer)
	return err
}

// Backup runs VACUUM INTO and optionally uploads to R2.
func Backup(ctx context.Context, envFile string) error {
	pools, cfg, err := openPools(envFile)
	if err != nil {
		return err
	}
	defer pools.Close()
	dest := cfg.DBPath + ".backup"
	if _, err := pools.Writer.ExecContext(ctx, `VACUUM INTO ?`, dest); err != nil {
		return err
	}
	fmt.Println("backup written:", dest)
	return nil
}

// Sync pushes outbox to D1/R2 (see pkg/sync). Prefer `cogs sync` from the project root.
func Sync(ctx context.Context, envFile string, args []string) error {
	return fmt.Errorf("sync: use `cogs sync` from the project directory (requires sync.json and COGS_D1_* / R2)")
}
