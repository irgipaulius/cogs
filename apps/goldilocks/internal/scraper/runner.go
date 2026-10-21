package scraper

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/irgipaulius/cogs/apps/goldilocks/internal/store"
)

// Runner invokes the Python chunk scraper on an interval.
type Runner struct {
	Store      *store.Store
	ScriptPath string
	TCPath     string
	DBPath     string
	Interval   time.Duration
}

func (r *Runner) Start(ctx context.Context) {
	if r.Interval <= 0 {
		r.Interval = 45 * time.Second
	}
	if r.ScriptPath == "" {
		r.ScriptPath = "scripts/scrape_chunk.py"
	}
	go func() {
		time.Sleep(5 * time.Second)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := r.runOnce(ctx); err != nil {
					log.Printf("scraper: %v", err)
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(r.Interval):
				}
			}
		}
	}()
}

func (r *Runner) runOnce(ctx context.Context) error {
	if _, err := os.Stat(r.ScriptPath); err != nil {
		return nil
	}
	py := "python3"
	args := []string{r.ScriptPath, "--db", r.DBPath}
	if r.TCPath != "" {
		args = append(args, "--tc", r.TCPath)
	}
	cmd := exec.CommandContext(ctx, py, args...)
	cmd.Dir = filepath.Dir(r.ScriptPath)
	if wd := os.Getenv("COGS_WORKDIR"); wd != "" {
		cmd.Dir = wd
	}
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		log.Printf("scraper: %s", string(out))
	}
	if err != nil {
		return err
	}
	return r.Store.BumpTileCacheRev(ctx)
}
