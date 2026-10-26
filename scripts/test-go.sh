#!/usr/bin/env bash
# Run go test with a module cache outside the workspace (go.work safe).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
WS="$ROOT"
if cache=$(go env GOMODCACHE 2>/dev/null); then
  case "$cache" in
    "$WS"/*|"$(realpath "$WS" 2>/dev/null || echo "$WS")"/*)
      export GOMODCACHE="${COGS_HOME:-${TMPDIR:-/tmp}/cogs-gomodcache}"
      mkdir -p "$GOMODCACHE"
      ;;
  esac
fi
exec go test "$@"
