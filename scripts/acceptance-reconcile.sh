#!/usr/bin/env bash
# CoGS reconcile acceptance — run on Cedevita (or any CoGS host) after deploy from main.
set -euo pipefail

ROOT="${COGS_ROOT:-$(pwd)}"
ADMIN="${COGS_ADMIN_URL:-http://127.0.0.1:8081}"
COOKIE="${COGS_ADMIN_COOKIE:-}"
PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }
skip() { echo "SKIP: $1 (manual on Cedevita)"; }

curl_api() {
  local method=$1 path=$2
  shift 2
  local args=(-sS -w "\n%{http_code}" -X "$method" -H "Content-Type: application/json")
  if [[ -n "$COOKIE" ]]; then
    args+=(-H "Cookie: $COOKIE")
  fi
  curl "${args[@]}" "$ADMIN$path" "$@"
}

echo "CoGS acceptance-reconcile (ROOT=$ROOT ADMIN=$ADMIN)"
echo "===================================================="

# 1 — remove/reinstall rampocalypse (manual)
skip "Test 1 — remove ram, install rampocalypse, Live + URLs"

# 2 — empty state after remove (manual)
skip "Test 2 — remove verify empty state + CF lists clean + reinstall"

# 3 — half-installed reconcile
if curl -sf "$ADMIN/health" >/dev/null 2>&1; then
  pass "Test 3a — admin /health reachable"
else
  fail "Test 3a — admin /health unreachable at $ADMIN"
fi

# 4 — breaking commit still serves last_good (manual)
skip "Test 4 — goldilocks breaking push still serves last_good"

# 5 — bad health release unchanged (manual)
skip "Test 5 — force bad health release leaves current unchanged"

# 6 — reboot jail cogs up (manual)
skip "Test 6 — reboot jail, cogs up, all present apps Live"

# 7 — duplicate repo rejected
if [[ -f "$ROOT/.cogs/state.db" ]]; then
  pass "Test 7a — state.db present"
else
  fail "Test 7a — missing $ROOT/.cogs/state.db"
fi

# 8 — hostname collision API
code=$(curl -sS -o /dev/null -w "%{http_code}" "$ADMIN/api/hostnames/check?subdomain=test&zone=example.com&skip_app=x" 2>/dev/null || echo "000")
if [[ "$code" == "401" || "$code" == "403" ]]; then
  pass "Test 8a — hostnames/check requires auth ($code)"
elif [[ "$code" == "200" ]]; then
  pass "Test 8a — hostnames/check responds 200 (root session may be required for full check)"
else
  fail "Test 8b — hostnames/check unexpected HTTP $code"
fi

# 9 — git status clean in CoGS checkout
if [[ -d "$ROOT/.git" ]]; then
  if [[ -z "$(git -C "$ROOT" status --porcelain 2>/dev/null)" ]]; then
    pass "Test 9 — CoGS git checkout clean"
  else
    fail "Test 9 — CoGS git checkout dirty (expected clean on deploy host checkout)"
  fi
else
  skip "Test 9 — not a git checkout"
fi

# 10 — queue pull no 401 in up.log
UPLOG="$ROOT/.cogs/run/up.log"
if [[ -f "$UPLOG" ]]; then
  if grep -E 'queue pull.*401|queue pull token.*401' "$UPLOG" >/dev/null 2>&1; then
    fail "Test 10 — 401 in up.log queue pull"
  else
    pass "Test 10 — no queue-pull 401 in up.log"
  fi
else
  skip "Test 10 — up.log missing (cogs up not run yet)"
fi

# 11 — UI errors visible (manual)
skip "Test 11 — admin UI surfaces API errors on failed actions"

# 12 — up.log bounded
if [[ -f "$UPLOG" ]]; then
  sz=$(stat -f%z "$UPLOG" 2>/dev/null || stat -c%s "$UPLOG" 2>/dev/null || echo 0)
  if (( sz <= 2500000 )); then
    pass "Test 12 — up.log size ${sz} bytes (rotation at 2MB)"
  else
    fail "Test 12 — up.log ${sz} bytes exceeds 2.5MB (rotation broken)"
  fi
else
  skip "Test 12 — up.log missing"
fi

echo "===================================================="
echo "SUMMARY: $PASS passed, $FAIL failed (manual tests listed as SKIP)"
if (( FAIL > 0 )); then
  exit 1
fi
exit 0
