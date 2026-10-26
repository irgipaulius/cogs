package appcfg

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	BackupDriverSQLite = "sqlite"
	BackupDriverCommand = "command"
)

// Backup settings from cogs.app.toml (defaults applied in NormalizeBackup).
type Backup struct {
	Enabled        bool
	Every          time.Duration
	Keep           int
	Dir            string
	Driver         string
	DBRelPath      string
	SnapshotArgv   []string
	RestoreArgv    []string
}

// NormalizeBackup fills defaults and validates backup settings.
func (a *App) NormalizeBackup() (Backup, error) {
	b := a.Backup
	if !b.Enabled {
		return b, nil
	}
	if b.Every <= 0 {
		b.Every = 24 * time.Hour
	}
	if b.Every < 15*time.Minute {
		return Backup{}, fmt.Errorf("backup_every must be at least 15m")
	}
	if b.Keep <= 0 {
		b.Keep = 7
	}
	if b.Keep > 90 {
		return Backup{}, fmt.Errorf("backup_keep must be at most 90")
	}
	if strings.TrimSpace(b.Dir) == "" {
		b.Dir = ".cogs/backups"
	}
	if strings.TrimSpace(b.Driver) == "" {
		b.Driver = BackupDriverSQLite
	}
	switch b.Driver {
	case BackupDriverSQLite:
		if len(b.SnapshotArgv) > 0 || len(b.RestoreArgv) > 0 {
			return Backup{}, fmt.Errorf("backup_snapshot/backup_restore are only for backup_driver command")
		}
	case BackupDriverCommand:
		if len(b.SnapshotArgv) == 0 {
			return Backup{}, fmt.Errorf("backup_snapshot argv required for command driver")
		}
		if len(b.RestoreArgv) == 0 {
			return Backup{}, fmt.Errorf("backup_restore argv required for command driver")
		}
	default:
		return Backup{}, fmt.Errorf("backup_driver must be sqlite or command")
	}
	if strings.Contains(b.Dir, "..") {
		return Backup{}, fmt.Errorf("backup_dir must stay inside the app directory")
	}
	return b, nil
}

func parseJSONArgv(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON argv: %w", err)
	}
	return out, nil
}
