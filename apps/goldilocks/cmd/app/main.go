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

	"github.com/irgipaulius/cogs/apps/goldilocks/internal/api"
	"github.com/irgipaulius/cogs/apps/goldilocks/internal/grid"
	"github.com/irgipaulius/cogs/apps/goldilocks/internal/pack"
	"github.com/irgipaulius/cogs/apps/goldilocks/internal/db"
	"github.com/irgipaulius/cogs/apps/goldilocks/internal/scraper"
	"github.com/irgipaulius/cogs/apps/goldilocks/internal/seed"
	"github.com/irgipaulius/cogs/apps/goldilocks/internal/store"
	"github.com/irgipaulius/cogs/pkg/blobstore"
	"github.com/irgipaulius/cogs/pkg/cockpit"
	"github.com/irgipaulius/cogs/pkg/config"
	"github.com/irgipaulius/cogs/pkg/cors"
	"github.com/irgipaulius/cogs/pkg/limiter"
	"github.com/irgipaulius/cogs/pkg/observe"
	"github.com/irgipaulius/cogs/pkg/r2"
	"github.com/irgipaulius/cogs/pkg/server"
	"github.com/irgipaulius/cogs/pkg/session"
	"github.com/irgipaulius/cogs/pkg/sqliteopen"
	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	if cmd == "version" {
		fmt.Println("goldilocks 0.1.0")
		return
	}
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	envFile := fs.String("env", ".env", "env file path")
	source := fs.String("source", "", "legacy sqlite to import")
	areas := fs.String("areas", "", "areas.json for country index")
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
	case "seed":
		if err := runSeed(ctx, *envFile, *source, *areas); err != nil {
			log.Fatal(err)
		}
	case "index-countries":
		if err := runIndexCountries(ctx, *envFile, *areas); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: app serve|migrate|seed|index-countries|version [-env .env] [-source path] [-areas path]")
}

func runIndexCountries(ctx context.Context, envFile, areas string) error {
	pools, _, err := openDB(envFile)
	if err != nil {
		return err
	}
	defer pools.Close()
	if areas == "" {
		areas = filepath.Join("public", "data", "areas.json")
	}
	return seed.IndexCountries(ctx, &store.Store{DB: pools.Writer}, areas, grid.LevelEighth)
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

func runSeed(ctx context.Context, envFile, source, areas string) error {
	pools, _, err := openDB(envFile)
	if err != nil {
		return err
	}
	defer pools.Close()
	if _, err := db.MigrateUp(ctx, pools.Writer); err != nil {
		return err
	}
	st := &store.Store{DB: pools.Writer}
	if source == "" {
		source = seed.DefaultSourcePath()
	}
	if areas == "" {
		areas = filepath.Join("public", "data", "areas.json")
	}
	return seed.ImportFromLegacy(ctx, st, source, areas)
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

	st := &store.Store{DB: pools.Writer}
	if err := pack.EnsurePacked(ctx, st); err != nil {
		log.Printf("climate tile pack: %v", err)
	}
	need, err := seed.NeedsSeed(ctx, st)
	if err != nil {
		return err
	}
	if need {
		src := os.Getenv("COGS_SEED_SQLITE")
		if src == "" {
			src = seed.DefaultSourcePath()
		}
		areas := os.Getenv("COGS_AREAS_JSON")
		if areas == "" {
			areas = "public/data/areas.json"
		}
		if _, err := os.Stat(src); err == nil {
			log.Printf("auto-seed from %s", src)
			if err := seed.ImportFromLegacy(ctx, st, src, areas); err != nil {
				log.Printf("seed failed: %v", err)
			}
		} else {
			log.Printf("no seed file at %s (map empty until seed)", src)
		}
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
			r2c = nil
		}
	}
	blobs, err := blobstore.New(ctx, cfg, r2c)
	if err != nil {
		return fmt.Errorf("blob store: %w", err)
	}

	climateAPI := &api.API{
		Store: &store.Store{DB: pools.Reader},
		Blobs: api.BlobCacheFrom(blobs),
	}

	publicDir := os.Getenv("COGS_PUBLIC_DIR")
	if publicDir == "" {
		publicDir = "public"
	}
	fileServer := http.FileServer(http.Dir(publicDir))

	origins := []string{"http://127.0.0.1:18788", "http://localhost:18788"}
	if cfg.PublicPageOrigin != "" {
		origins = append(origins, cfg.PublicPageOrigin)
	}

	wd, _ := os.Getwd()
	tel := cockpit.FromConfig(cfg, wd)
	mux := server.NewMux(server.Deps{
		Config:       cfg,
		DB:           pools.Reader,
		Session:      &session.Manager{Secret: []byte(cfg.SessionSecret), TurnstileSecret: cfg.TurnstileSecret},
		IPLimiter:    limiter.NewLRU(20_000, 30, 60),
		MigrationVer: ver,
		Observe:      obs,
		Cockpit:      tel,
		RegisterRoutes: func(m *http.ServeMux) {
			climateAPI.Register(m)
			m.Handle("GET /", fileServer)
			m.Handle("GET /index.html", fileServer)
		},
	})
	handler := cors.Middleware(origins)(mux)

	scrapeCtx, scrapeCancel := context.WithCancel(context.Background())
	defer scrapeCancel()
	scriptPath := os.Getenv("COGS_SCRAPER_SCRIPT")
	if scriptPath == "" {
		scriptPath = "scripts/scrape_chunk.py"
	}
	(&scraper.Runner{
		Store:      st,
		ScriptPath: scriptPath,
		TCPath:     os.Getenv("COGS_TC_PATH"),
		DBPath:     cfg.DBPath,
		Interval:   45 * time.Second,
	}).Start(scrapeCtx)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Minute,
		WriteTimeout:      30 * time.Minute,
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		slog.Info("listening", "addr", cfg.ListenAddr, "public", publicDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-sig
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}
