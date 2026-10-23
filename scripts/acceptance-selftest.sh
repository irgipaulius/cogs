#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=acceptance-lib.sh
source "$SCRIPT_DIR/acceptance-lib.sh"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
log="$tmp/up.log"
printf 'queue pull token: old error\ncogs up start pid=9 at=2026-01-01T00:00:00Z\nqueue pull: ok\n' > "$log"
out=$(acceptance_up_log_since_start "$log")
[[ "$out" == *"queue pull: ok"* ]] || { echo "missing ok: $out"; exit 1; }
[[ "$out" != *"old error"* ]] || { echo "included old: $out"; exit 1; }

applog="$tmp/app.log"
mark="$tmp/app.log.start"
printf 'panic: old release\nsecond start text\n' > "$applog"
echo 18 > "$mark"
if acceptance_app_log_panic_since_start "$applog" "$mark"; then
  echo "should ignore pre-mark panic"
  exit 1
fi
printf 'panic: old release\nsecond start text\npanic: fresh\n' > "$applog"
if ! acceptance_app_log_panic_since_start "$applog" "$mark"; then
  echo "expected panic after mark"
  exit 1
fi

ADMIN="http://127.0.0.1:18786"
root="$tmp"
mkdir -p "$root/.cogs/run"
printf 'abc123token\n' > "$root/.cogs/run/acceptance.token"
if ! acceptance_load_loopback_token "$root"; then
  echo "load loopback token failed"
  exit 1
fi
[[ "$COOKIE" == *"abc123token"* ]] || { echo "bad cookie $COOKIE"; exit 1; }

echo "acceptance-selftest ok"
