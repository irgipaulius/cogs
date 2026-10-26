# Backups

Per-app **SQLite snapshots** (or a custom command) while **`cogs up`** runs. Snapshots are cataloged in workspace **`.cogs/state.db`**; files live under each app (default **`.cogs/backups`**). Off until you set **`backup = true`** in that app’s **`cogs.app.toml`**.

## Config reference (`cogs.app.toml`)

| Key | Default | Notes |
|-----|---------|--------|
| `backup` | `false` | Enable scheduled + manual backups |
| `backup_every` | `24h` | Go duration; minimum `15m` |
| `backup_keep` | `7` | Retained snapshots (max `90`) |
| `backup_dir` | `.cogs/backups` | Relative to app dir; must not escape the app tree |
| `backup_driver` | `sqlite` | `sqlite` or `command` |
| `backup_db` | *(from `.env`)* | Optional relative path; else `COGS_DB_PATH` / `COGS_ADMIN_DB_PATH` |
| `backup_snapshot` | — | JSON argv; **command** driver only |
| `backup_restore` | — | JSON argv; **command** driver only |

See also: [deploy.md](deploy.md) (host loop), [admin-dashboard.md](admin-dashboard.md) (root UI).

## Enable (SQLite apps)

Add to `cogs.app.toml`:

```toml
backup = true
backup_every = "24h"
backup_keep = 7
backup_dir = ".cogs/backups"
# optional: backup_db = "data/todo.db"
```

CoGS runs `VACUUM INTO` on the app database on a schedule while `cogs up` is running, catalogs snapshots in `state.db`, and prunes old files.

## CLI (host / SSH)

```bash
cogs backup now todo
cogs backup list todo
cogs backup restore todo 42 --yes   # global -y / --yes also works
```

Restore stops the app, swaps the database file, and starts the app again.

## Admin UI

App detail → **Backups** tab (root only): list snapshots, **Backup now**, **Restore** (confirm dialog). The API never accepts raw filesystem paths — only catalog ids.

## Custom database (command driver)

For non-SQLite apps, use a script inside the app tree (no shell):

```toml
backup = true
backup_driver = "command"
backup_snapshot = '["./scripts/backup"]'
backup_restore = '["./scripts/restore"]'
```

CoGS sets `COGS_APP_DIR`, `COGS_BACKUP_DEST` (snapshot), or `COGS_BACKUP_SRC` (restore). Binaries must live under the app directory.

## Verify a file

```bash
cogs restore-test path/to/snapshot.db
```

Runs `PRAGMA integrity_check`.

## Litestream

Optional continuous replication: see `deploy/litestream.yml.tmpl`.
