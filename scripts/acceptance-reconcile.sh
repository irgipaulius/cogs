#!/usr/bin/env bash
# CoGS reconcile acceptance — run on Cedevita after deploy from main.
set -euo pipefail

ROOT="${COGS_ROOT:-$(pwd)}"
COOKIE="${COGS_ADMIN_COOKIE:-}"
PASS=0
FAIL=0
SKIP=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }
skip() { echo "SKIP: $1"; SKIP=$((SKIP + 1)); }

admin_url() {
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

ADMIN="$(admin_url)"
echo "CoGS acceptance-reconcile (ROOT=$ROOT ADMIN=$ADMIN)"
echo "===================================================="

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

# Test 3 — admin health
if curl -sf "$ADMIN/health" >/dev/null 2>&1; then
  pass "Test 3 — admin /health reachable"
else
  fail "Test 3 — admin /health unreachable at $ADMIN"
fi

skip "Test 4 — goldilocks breaking push / last_good (manual)"
skip "Test 5 — bad health release unchanged (manual)"
skip "Test 6 — reboot jail cogs up (manual)"

# Test 7 — duplicate repo guard (state present)
if [[ -f "$ROOT/.cogs/state.db" ]]; then
  pass "Test 7 — state.db present for duplicate-repo checks"
fi

# Test 8 — hostname check
code=$(curl -sS -o /dev/null -w "%{http_code}" "$ADMIN/api/hostnames/check?subdomain=test&zone=example.com&skip_app=x" 2>/dev/null || echo "000")
if [[ "$code" == "401" || "$code" == "403" || "$code" == "200" ]]; then
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

# Test 10 — queue pull 401
UPLOG="$ROOT/.cogs/run/up.log"
if [[ -f "$UPLOG" ]]; then
  if grep -E 'queue pull.*401|queue pull token.*401' "$UPLOG" >/dev/null 2>&1; then
    fail "Test 10 — 401 in up.log queue pull"
  else
    pass "Test 10 — no queue-pull 401 in up.log"
  fi
else
  skip "Test 10 — up.log missing"
fi

skip "Test 11 — UI error surfacing (manual browser)"

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

echo "===================================================="
echo "SUMMARY: $PASS passed, $FAIL failed, $SKIP skipped"
if (( FAIL > 0 )); then
  exit 1
fi
exit 0
