package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/irgipaulius/cogs/pkg/config"
	"github.com/irgipaulius/cogs/pkg/r2"
	pkgsync "github.com/irgipaulius/cogs/pkg/sync"
	"github.com/irgipaulius/cogs/pkg/sqliteopen"
)

func cmdSyncSchema(args []string) error {
	fs := flag.NewFlagSet("sync schema", flag.ExitOnError)
	envFile := fs.String("env", ".env", "env file")
	_ = fs.Parse(args)
	cfg, err := config.LoadRelaxed(*envFile)
	if err != nil {
		return err
	}
	pools, err := sqliteopen.Open(sqliteopen.Config{Path: cfg.DBPath})
	if err != nil {
		return err
	}
	defer pools.Close()
	ctx := context.Background()
	if err := pkgsync.EnsureSchema(ctx, pools.Writer); err != nil {
		return err
	}
	fmt.Println("local sync schema ok (_cogs_outbox, _cogs_blocks)")
	fmt.Println("create D1 tables from sync.json manually or via your first sync migration")
	return nil
}

func cmdSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	envFile := fs.String("env", ".env", "env file")
	del := fs.Bool("delete", false, "apply deletes to D1")
	checksum := fs.Bool("checksum", false, "block checksum drift check")
	rebuild := fs.Bool("rebuild", false, "rescan table into outbox (slow)")
	set := fs.String("set", "items", "sync set name from sync.json")
	_ = fs.Parse(args)

	cfg, err := config.LoadRelaxed(*envFile)
	if err != nil {
		return err
	}
	syncCfg, err := pkgsync.LoadConfig("sync.json")
	if err != nil {
		return err
	}
	pools, err := sqliteopen.Open(sqliteopen.Config{Path: cfg.DBPath, ReaderConns: 4})
	if err != nil {
		return err
	}
	defer pools.Close()
	ctx := context.Background()
	if err := pkgsync.EnsureSchema(ctx, pools.Writer); err != nil {
		return err
	}

	var d1 pkgsync.D1Client
	if cfg.D1AccountID != "" && cfg.D1DatabaseID != "" && cfg.D1APIToken != "" {
		d1 = &pkgsync.HTTPD1{
			AccountID:  cfg.D1AccountID,
			DatabaseID: cfg.D1DatabaseID,
			Token:      cfg.D1APIToken,
		}
	}

	var blobs pkgsync.BlobStore
	if cfg.R2AccountID != "" {
		r2c, err := r2.New(ctx, r2.Config{
			AccountID:       cfg.R2AccountID,
			AccessKeyID:     cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey,
			Bucket:          cfg.R2Bucket,
		})
		if err != nil {
			return err
		}
		blobs = &r2BlobAdapter{client: r2c}
	}

	eng := pkgsync.Engine{
		DB: pools.Writer, D1: d1, Blobs: blobs,
		Config: syncCfg, SetName: *set, AllowDel: *del,
	}

	if *rebuild {
		n, err := eng.RebuildOutbox(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("rebuild: queued %d rows\n", n)
	}
	var st pkgsync.Stats
	var errRun error
	if *checksum {
		st, errRun = eng.ChecksumSync(ctx)
	} else {
		st, errRun = eng.PushOutbox(ctx)
	}
	if errRun != nil {
		return errRun
	}
	fmt.Printf("sync: dirty=%d uploaded=%d skipped=%d deleted=%d\n", st.Dirty, st.Uploaded, st.Skipped, st.Deleted)
	return nil
}

type r2BlobAdapter struct {
	client *r2.Client
}

func (a *r2BlobAdapter) HeadHash(ctx context.Context, key string) (string, bool, error) {
	h, ok, err := a.client.HeadSHA256(ctx, key)
	return h, ok, err
}

func (a *r2BlobAdapter) PutBlob(ctx context.Context, key string, data []byte) (string, error) {
	return a.client.PutCompressed(ctx, key, data)
}
