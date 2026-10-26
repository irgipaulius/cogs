package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/irgipaulius/cogs/pkg/appcfg"
	"github.com/irgipaulius/cogs/pkg/state"
)

// Service runs snapshots and restores for one app.
type Service struct {
	WSRoot string
	Store  *state.Store
}

// Due reports whether a scheduled backup should run now.
func Due(last time.Time, every time.Duration, now time.Time) bool {
	if every <= 0 {
		return false
	}
	if last.IsZero() {
		return true
	}
	return !now.Before(last.Add(every))
}

// Snapshot creates a backup for appID at appDir using cogs.app.toml settings.
func (s *Service) Snapshot(ctx context.Context, appID, appDir string) (int64, error) {
	app, err := appcfg.Load(appDir)
	if err != nil {
		return 0, err
	}
	cfg, err := app.NormalizeBackup()
	if err != nil {
		return 0, err
	}
	if !cfg.Enabled {
		return 0, fmt.Errorf("backup disabled for %s", appID)
	}
	if err := acquireAppLock(appDir); err != nil {
		return 0, err
	}
	defer releaseAppLock(appDir)

	ts := time.Now().UTC().Format("20060102T150405Z")
	rel := SnapshotRelPath(cfg.Dir, ts)
	destAbs, err := ResolveInsideApp(appDir, rel)
	if err != nil {
		return 0, err
	}

	switch cfg.Driver {
	case appcfg.BackupDriverSQLite:
		dbRel := cfg.DBRelPath
		if dbRel == "" {
			dbRel, err = DBPathFromEnv(appDir)
			if err != nil {
				return 0, err
			}
		}
		dbAbs, err := ResolveInsideApp(appDir, dbRel)
		if err != nil {
			return 0, err
		}
		if err := SnapshotSQLite(ctx, dbAbs, destAbs); err != nil {
			return 0, err
		}
	case appcfg.BackupDriverCommand:
		if err := RunCommandDriver(ctx, appDir, cfg.SnapshotArgv, map[string]string{
			"COGS_APP_DIR":       appDir,
			"COGS_BACKUP_DEST":   destAbs,
			"COGS_BACKUP_REL":    rel,
		}); err != nil {
			return 0, err
		}
		if err := IntegrityCheckFile(ctx, destAbs); err != nil {
			return 0, err
		}
	default:
		return 0, fmt.Errorf("unknown driver")
	}

	st, err := os.Stat(destAbs)
	if err != nil {
		return 0, err
	}
	sum, err := fileSHA256(destAbs)
	if err != nil {
		return 0, err
	}
	_ = os.Chmod(destAbs, 0o600)
	id, err := s.Store.InsertBackup(appID, rel, cfg.Driver, st.Size(), sum)
	if err != nil {
		return 0, err
	}
	_ = s.prune(appID, appDir, cfg.Keep)
	return id, nil
}

func (s *Service) prune(appID, appDir string, keep int) error {
	ids, err := s.Store.ListBackupIDsOlderThan(appID, keep)
	if err != nil {
		return err
	}
	for _, id := range ids {
		row, err := s.Store.GetBackup(appID, id)
		if err != nil {
			continue
		}
		if abs, err := ResolveInsideApp(appDir, row.RelPath); err == nil {
			_ = os.Remove(abs)
		}
		_ = s.Store.DeleteBackupRow(id)
	}
	return nil
}

// Restore replaces the live database from catalog id.
func (s *Service) Restore(ctx context.Context, appID, appDir string, backupID int64, stopStart func() error, start func() error) error {
	app, err := appcfg.Load(appDir)
	if err != nil {
		return err
	}
	cfg, err := app.NormalizeBackup()
	if err != nil {
		return err
	}
	row, err := s.Store.GetBackup(appID, backupID)
	if err != nil {
		return err
	}
	snapAbs, err := ResolveInsideApp(appDir, row.RelPath)
	if err != nil {
		return err
	}
	if err := IntegrityCheckFile(ctx, snapAbs); err != nil {
		return err
	}
	if err := acquireAppLock(appDir); err != nil {
		return err
	}
	defer releaseAppLock(appDir)

	dbRel := cfg.DBRelPath
	if dbRel == "" {
		dbRel, _ = DBPathFromEnv(appDir)
	}
	liveAbs, err := ResolveInsideApp(appDir, dbRel)
	if err != nil {
		return err
	}

	if stopStart != nil {
		if err := stopStart(); err != nil {
			return err
		}
	}

	switch cfg.Driver {
	case appcfg.BackupDriverSQLite:
		pre := liveAbs + ".pre-restore-" + time.Now().UTC().Format("20060102T150405Z")
		if _, err := os.Stat(liveAbs); err == nil {
			if err := os.Rename(liveAbs, pre); err != nil {
				return err
			}
		}
		if err := copyFile(snapAbs, liveAbs); err != nil {
			_ = os.Rename(pre, liveAbs)
			return err
		}
		_ = os.Chmod(liveAbs, 0o600)
		if err := IntegrityCheckFile(ctx, liveAbs); err != nil {
			_ = os.Remove(liveAbs)
			_ = os.Rename(pre, liveAbs)
			return err
		}
	case appcfg.BackupDriverCommand:
		if err := RunCommandDriver(ctx, appDir, cfg.RestoreArgv, map[string]string{
			"COGS_APP_DIR":     appDir,
			"COGS_BACKUP_SRC":  snapAbs,
			"COGS_BACKUP_REL":  row.RelPath,
		}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown driver")
	}

	if start != nil {
		return start()
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
