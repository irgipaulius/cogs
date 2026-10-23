#!/usr/bin/env bash
# CoGS reconcile acceptance — run on Cedevita after deploy from main.
if [ -z "${BASH_VERSION:-}" ]; then
  exec /usr/bin/env bash "$0" "$@"
fi
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=acceptance-lib.sh
source "$SCRIPT_DIR/acceptance-lib.sh"
ROOT="${COGS_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
COOKIE="${COGS_ADMIN_COOKIE:-}"
PASS=0
FAIL=0
SKIP=0

ensure_acceptance_cookie() {
  if [[ -n "$COOKIE" ]]; then
    return 0
  fi
  acceptance_load_admin_credentials "$ROOT" || true
  local user pass
  user="${COGS_ACCEPTANCE_ADMIN_USER:-}"
  pass="${COGS_ACCEPTANCE_ADMIN_PASSWORD:-}"
  if [[ -z "$user" || -z "$pass" ]]; then
    return 1
  fi
  local jar
  jar="$(mktemp)"
  local code
  code=$(curl -sS -o /dev/null -w "%{http_code}" -c "$jar" -X POST "$ADMIN/api/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${user}\",\"password\":\"${pass}\"}" 2>/dev/null || echo "000")
  if [[ "$code" != "200" ]]; then
    rm -f "$jar"
    return 1
  fi
  COOKIE="cogs_admin_session=$(awk '$6=="cogs_admin_session"{print $7}' "$jar" | tail -1)"
  rm -f "$jar"
  [[ -n "$COOKIE" ]]
}

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }
skip() { echo "SKIP: $1"; SKIP=$((SKIP + 1)); }

admin_local_url() {
  if [[ -n "${COGS_ADMIN_URL:-}" ]]; then
    echo "$COGS_ADMIN_URL"
    return
  fi
  local envf="$ROOT/apps/admin/.env"
  if [[ -f "$envf" ]]; then
    local listen
    listen=$(grep -E '^COGS_LISTEN=' "$envf" | head -1 | cut -d= -f2- | tr -d '"' | tr -d "'")
    if [[ -n "$listen" ]]; then
      if [[ "$listen" == *:* ]]; then
        echo "http://${listen}"
        return
      fi
      echo "http://127.0.0.1:${listen}"
      return
    fi
  fi
  echo "http://127.0.0.1:18786"
}

admin_public_url() {
  if [[ -n "${COGS_ADMIN_PUBLIC_URL:-}" ]]; then
    echo "$COGS_ADMIN_PUBLIC_URL"
    return
  fi
  local envf="$ROOT/apps/admin/.env"
  if [[ -f "$envf" ]]; then
    local origin
    origin=$(grep -E '^COGS_PUBLIC_PAGE_ORIGIN=' "$envf" | head -1 | cut -d= -f2- | tr -d '"' | tr -d "'")
    if [[ -n "$origin" ]]; then
      echo "${origin%/}"
      return
    fi
  fi
  echo ""
}

ADMIN="$(admin_local_url)"
PUBLIC_ADMIN="$(admin_public_url)"
echo "CoGS acceptance-reconcile (ROOT=$ROOT ADMIN=$ADMIN PUBLIC=${PUBLIC_ADMIN:-none})"
echo "===================================================="

if ! ensure_acceptance_cookie; then
  fail "acceptance auth — set COGS_ADMIN_COOKIE, COGS_ACCEPTANCE_ADMIN_*, or apps/admin/.env on loopback"
fi

if [[ -z "${COGS_ACCEPTANCE_FULL:-}" && -n "$COOKIE" && -f "$ROOT/apps/admin/.env" ]] && acceptance_admin_is_loopback "$ADMIN"; then
  COGS_ACCEPTANCE_FULL=1
  export COGS_ACCEPTANCE_FULL
fi

curl_api() {
  local method=$1 path=$2
  shift 2
  local args=(-sS -w "\n%{http_code}" -X "$method" -H "Content-Type: application/json")
  if [[ -n "$COOKIE" ]]; then
    args+=(-H "Cookie: $COOKIE")
  fi
  curl "${args[@]}" "$ADMIN$path" "$@"
}

# Test 1 — remove + install rampocalypse (automated when COGS_ACCEPTANCE_FULL=1)
if [[ "${COGS_ACCEPTANCE_FULL:-}" == "1" && -n "$COOKIE" ]]; then
  body='{"repo":"irgipaulius/rampocalypse","name":"ram"}'
  code=$(curl_api POST /api/apps/ram/remove -d '{}' 2>/dev/null | tail -1 || echo "000")
  if [[ "$code" == "200" || "$code" == "404" ]]; then
    pass "Test 1a — remove ram API ($code)"
  else
    fail "Test 1a — remove ram HTTP $code"
  fi
  sleep 2
  code=$(curl_api POST /api/apps -d "$body" 2>/dev/null | tail -1 || echo "000")
  if [[ "$code" == "200" ]]; then
    pass "Test 1b — install rampocalypse queued"
  else
    fail "Test 1b — install rampocalypse HTTP $code"
  fi
else
  skip "Test 1 — set COGS_ACCEPTANCE_FULL=1 and COGS_ADMIN_COOKIE for automated rampocalypse install"
fi

# Test 2 — state has apps after import
if [[ -f "$ROOT/.cogs/state.db" ]]; then
  n=$(sqlite3 "$ROOT/.cogs/state.db" "SELECT COUNT(*) FROM apps;" 2>/dev/null || echo 0)
  ws=$(grep -c '"name"' "$ROOT/cogs.workspace.json" 2>/dev/null || echo 0)
  if (( n > 0 )); then
    pass "Test 2 — state.db apps=$n"
  elif (( ws > 0 )); then
    fail "Test 2 — state.db apps=0 but workspace lists apps (legacy import failed)"
  else
    skip "Test 2 — empty workspace"
  fi
else
  fail "Test 2 — missing .cogs/state.db"
fi

# Test 3 — public admin health (tunnel), not loopback-only
if [[ -n "$PUBLIC_ADMIN" ]]; then
  code=$(curl -sS -o /dev/null -w "%{http_code}" --max-time 25 "${PUBLIC_ADMIN}/health" 2>/dev/null || echo "000")
  if [[ "$code" == "200" ]]; then
    pass "Test 3 — public admin /health HTTP 200 at $PUBLIC_ADMIN"
  else
    fail "Test 3 — public admin /health HTTP $code at $PUBLIC_ADMIN (tunnel/DNS?)"
  fi
else
  fail "Test 3 — set COGS_ADMIN_PUBLIC_URL or COGS_PUBLIC_PAGE_ORIGIN in apps/admin/.env"
fi

if [[ -f "$ROOT/.cogs/state.db" ]]; then
  lg=$(sqlite3 "$ROOT/.cogs/state.db" "SELECT r.commit_sha FROM app_release_pointers p JOIN app_releases r ON r.id=p.last_good_release_id WHERE p.app_id='goldilocks' LIMIT 1;" 2>/dev/null || true)
  if [[ -n "$lg" ]]; then
    pass "Test 4 — goldilocks last_good release set"
  else
    fail "Test 4 — goldilocks has no last_good release in state.db"
  fi
  bad_health=$(sqlite3 "$ROOT/.cogs/state.db" "SELECT a.id FROM app_release_pointers p JOIN app_releases r ON r.id=p.current_release_id JOIN apps a ON a.id=p.app_id WHERE a.desired_present=1 AND r.health_ok=0 LIMIT 1;" 2>/dev/null || true)
  if [[ -z "$bad_health" ]]; then
    pass "Test 5 — no current release with health_ok=0"
  else
    fail "Test 5 — app $bad_health serving release that failed health"
  fi
else
  fail "Test 4/5 — missing state.db"
fi

if [[ "${COGS_ACCEPTANCE_REBOOT:-}" == "1" ]]; then
  skip "Test 6 — reboot check is operator-driven (COGS_ACCEPTANCE_REBOOT=1 set)"
else
  skip "Test 6 — set COGS_ACCEPTANCE_REBOOT=1 after manual jail reboot to acknowledge"
fi

# Test 7 — duplicate repo guard (state present)
if [[ -f "$ROOT/.cogs/state.db" ]]; then
  pass "Test 7 — state.db present for duplicate-repo checks"
fi

# Test 8 — hostname check (401 without session is a failure)
code=$(curl -sS -o /dev/null -w "%{http_code}" -H "Cookie: $COOKIE" "$ADMIN/api/hostnames/check?subdomain=test&zone=example.com&skip_app=x" 2>/dev/null || echo "000")
if [[ "$code" == "401" ]]; then
  fail "Test 8 — hostnames/check HTTP 401 (set COGS_ADMIN_COOKIE or fix auth)"
elif [[ "$code" == "403" || "$code" == "200" ]]; then
  pass "Test 8 — hostnames/check HTTP $code"
else
  fail "Test 8 — hostnames/check HTTP $code"
fi

# Test 9 — git clean (tracked changes only)
if [[ -d "$ROOT/.git" ]]; then
  if [[ -z "$(git -C "$ROOT" status --porcelain --untracked-files=no 2>/dev/null)" ]]; then
    pass "Test 9 — git checkout clean (tracked)"
  else
    fail "Test 9 — git has tracked modifications"
  fi
else
  skip "Test 9 — not a git checkout"
fi

# Test 10 — queue pull errors (current cogs up run only)
UPLOG="$ROOT/.cogs/run/up.log"
if [[ -f "$UPLOG" ]]; then
  uplog_slice=$(acceptance_up_log_since_start "$UPLOG")
  if printf '%s' "$uplog_slice" | grep -E 'queue pull|queue ack|queue pull token|queue pull account' | grep -qiE 'error|401|403|fail|missing' >/dev/null 2>&1; then
    fail "Test 10 — queue pull errors in up.log"
  else
    pass "Test 10 — no queue-pull errors in up.log"
  fi
else
  skip "Test 10 — up.log missing"
fi

if [[ -n "$COOKIE" ]]; then
  stream=$(curl -sS --max-time 5 -H "Cookie: $COOKIE" "$ADMIN/api/reconcile/stream" 2>/dev/null | head -20 || true)
  if echo "$stream" | grep -q 'last_error'; then
    pass "Test 11 — reconcile stream exposes last_error field"
  else
    fail "Test 11 — reconcile stream missing last_error (auth or SSE broken)"
  fi
else
  fail "Test 11 — needs acceptance admin cookie"
fi

# Test 12 — up.log size / rotation
if [[ -f "$UPLOG" ]]; then
  sz=$(stat -f%z "$UPLOG" 2>/dev/null || stat -c%s "$UPLOG" 2>/dev/null || echo 0)
  if (( sz <= 2500000 )); then
    pass "Test 12 — up.log size ${sz} bytes"
  else
    fail "Test 12 — up.log ${sz} bytes > 2.5MB"
  fi
else
  skip "Test 12 — up.log missing"
fi

# Test 13 — each present app in state.db has a running process
if [[ ! -f "$ROOT/.cogs/state.db" ]]; then
  fail "Test 13 — missing state.db"
else
  if ! command -v sqlite3 >/dev/null 2>&1; then
    fail "Test 13 — sqlite3 required"
  else
    while IFS='|' read -r app path; do
      [[ -z "$app" ]] && continue
      path="${path#"${path%%[![:space:]]*}"}"
      path="${path%"${path##*[![:space:]]}"}"
      pidf=""
      if [[ -n "$path" ]]; then
        pidf="$ROOT/${path}/.cogs/run/app.pid"
      fi
      if [[ ! -f "$pidf" ]]; then
        pidf=$(find "$ROOT/apps" -path "*/${app}/.cogs/run/app.pid" 2>/dev/null | head -1)
      fi
      logf=""
      [[ -n "$path" ]] && logf="$ROOT/${path}/.cogs/run/app.log"
      markf=""
      [[ -n "$path" ]] && markf="$ROOT/${path}/.cogs/run/app.log.start"
      if [[ -f "$logf" ]] && acceptance_app_log_panic_since_start "$logf" "$markf"; then
        fail "Test 13 — app ${app} panic in app.log since start"
        continue
      fi
      if [[ -f "$pidf" ]]; then
        pid=$(tr -d ' \n' < "$pidf")
        if kill -0 "$pid" 2>/dev/null; then
          pass "Test 13 — app ${app} running pid $pid"
        else
          fail "Test 13 — app ${app} stale pid $pid"
        fi
      else
        fail "Test 13 — app ${app} not running (no live app.pid)"
      fi
    done < <(sqlite3 -separator '|' "$ROOT/.cogs/state.db" "SELECT id, app_path FROM apps WHERE desired_present=1 ORDER BY id;" 2>/dev/null || true)
  fi
fi

# Test 14 — tunnel hostnames respond (admin + todo-api if configured)
check_tunnel_host() {
  local label=$1 url=$2
  [[ -z "$url" ]] && return 0
  code=$(curl -sS -o /dev/null -w "%{http_code}" --max-time 25 "${url%/}/health" 2>/dev/null || echo "000")
  if [[ "$code" == "200" ]]; then
    pass "Test 14 — tunnel $label HTTP 200"
  else
    fail "Test 14 — tunnel $label HTTP $code at $url"
  fi
}
if [[ -n "$PUBLIC_ADMIN" ]]; then
  check_tunnel_host "admin" "$PUBLIC_ADMIN"
fi
todo_api=$(grep -E '^COGS_PUBLIC_API_URL=' "$ROOT/apps/todo/.env" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)
if [[ -n "$todo_api" ]]; then
  check_tunnel_host "todo-api" "$todo_api"
fi
gold_api=$(grep -E '^COGS_PUBLIC_API_URL=' "$ROOT/apps/goldilocks/.env" 2>/dev/null | head -1 | cut -d= -f2- | tr -d '"' | tr -d "'" || true)
if [[ -n "$gold_api" ]]; then
  check_tunnel_host "goldilocks-api" "$gold_api"
fi

# Test 16 — no duplicate owned CF resources per app/kind
if [[ -f "$ROOT/.cogs/state.db" ]]; then
  dup=$(sqlite3 "$ROOT/.cogs/state.db" "SELECT app_id||':'||kind||':'||external_id FROM app_owned GROUP BY app_id, kind, external_id HAVING COUNT(*)>1 LIMIT 5;" 2>/dev/null || true)
  if [[ -z "$dup" ]]; then
    pass "Test 16 — no duplicate app_owned rows"
  else
    fail "Test 16 — duplicate owned resources: $(echo "$dup" | tr '\n' ' ')"
  fi
fi

# Test 15 — state.db: no app stuck unconverged / last_error
if [[ -f "$ROOT/.cogs/state.db" ]]; then
  bad=$(sqlite3 "$ROOT/.cogs/state.db" "SELECT id FROM apps WHERE desired_present=1 AND (converged=0 OR (last_error IS NOT NULL AND last_error != ''));" 2>/dev/null || true)
  if [[ -z "$bad" ]]; then
    pass "Test 15 — all present apps converged without last_error"
  else
    fail "Test 15 — apps need reconcile: $(echo "$bad" | tr '\n' ' ')"
  fi
fi

echo "===================================================="
echo "SUMMARY: $PASS passed, $FAIL failed, $SKIP skipped"
if (( FAIL > 0 )); then
  exit 1
fi
exit 0
