package scraper

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

var pythonMissingLogged bool

func resolvePython() string {
	if v := strings.TrimSpace(os.Getenv("COGS_PYTHON")); v != "" {
		return v
	}
	for _, name := range []string{"python3", "python3.12", "python3.11", "python3.10"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	if !pythonMissingLogged {
		pythonMissingLogged = true
		log.Print("scraper: no python3 on PATH — set COGS_PYTHON or install python3")
	}
	return ""
}

func (r *Runner) Start(ctx context.Context) {
	if r.Interval <= 0 {
		r.Interval = 45 * time.Second
	}
	appRoot, _ := os.Getwd()
	r.ScriptPath = ResolveScriptPath(appRoot, r.ScriptPath)
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
	py := resolvePython()
	if py == "" {
		return nil
	}
	args := []string{r.ScriptPath, "--db", r.DBPath}
	if r.TCPath != "" {
		args = append(args, "--tc", r.TCPath)
	}
	cmd := exec.CommandContext(ctx, py, args...)
	cmd.Dir = filepath.Dir(r.ScriptPath)
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		log.Printf("scraper: %s", string(out))
	}
	if err != nil {
		return err
	}
	return r.Store.BumpTileCacheRev(ctx)
}
