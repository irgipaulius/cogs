# Backups

## Built-in snapshot

```bash
go run ./cmd/app backup -env .env
```

Runs `VACUUM INTO` to `data.db.backup`. Optionally upload with R2 credentials configured.

## Litestream (optional)

Use `deploy/litestream.yml.tmpl` to replicate WAL to R2 continuously.

## Verify

```bash
cogs restore-test data.db.backup
```

Runs `PRAGMA integrity_check` on the copy.

## Filesystem snapshots

On ZFS/btrfs/LVM, snapshot the directory containing `data.db` + `-wal` / `-shm`. WAL snapshots are crash-consistent.
