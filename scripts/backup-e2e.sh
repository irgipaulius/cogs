#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
go build -o /tmp/cogs-backup-e2e ./cmd/cogs
WS="$(mktemp -d)"
trap 'rm -rf "$WS"' EXIT
mkdir -p "$WS/apps/demo"
cat > "$WS/cogs.workspace.json" <<EOF
{"name":"test","apps":[{"name":"demo","path":"apps/demo"}]}
EOF
cat > "$WS/apps/demo/cogs.app.toml" <<EOF
name = "demo"
backup = true
backup_every = "24h"
backup_db = "data.db"
EOF
mkdir -p "$WS/apps/demo/data"
sqlite3 "$WS/apps/demo/data.db" 'CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT); INSERT INTO kv VALUES ("x","1");'
(
  cd "$WS"
  /tmp/cogs-backup-e2e backup now demo
)
sqlite3 "$WS/apps/demo/data.db" "UPDATE kv SET v='2';"
(
  cd "$WS"
  id=$(/tmp/cogs-backup-e2e backup list demo | awk 'NR==1{print $1}')
  /tmp/cogs-backup-e2e backup restore demo "$id" --yes
)
v=$(sqlite3 "$WS/apps/demo/data.db" 'SELECT v FROM kv WHERE k="x";')
[[ "$v" == "1" ]] || { echo "expected v=1 got $v"; exit 1; }
echo "backup-e2e ok"
