# Shared helpers for acceptance-reconcile.sh (also self-tested).
acceptance_up_log_since_start() {
  local path=$1
  [[ -f "$path" ]] || return 0
  awk 'BEGIN{p=0} /^cogs up start pid=/{buf=$0"\n"; p=1; next} p{buf=buf $0 "\n"} END{printf "%s", buf}' "$path"
}

acceptance_app_log_panic_since_start() {
  local logf=$1 markf=$2
  [[ -f "$logf" ]] || return 1
  local off=0
  if [[ -f "$markf" ]]; then
    off=$(tr -d ' \n' < "$markf" 2>/dev/null || echo 0)
  fi
  if ! tail -c +"$((off + 1))" "$logf" 2>/dev/null | grep -q "panic:"; then
    return 1
  fi
  return 0
}

acceptance_admin_is_loopback() {
  local url=$1
  [[ "$url" =~ ^https?://(127\.0\.0\.1|localhost|\[::1\])(:[0-9]+)?(/|$) ]]
}

acceptance_load_admin_credentials() {
  local root=${1:-}
  [[ -n "${COGS_ADMIN_COOKIE:-}" || -n "${COGS_ACCEPTANCE_ADMIN_USER:-}" ]] && return 0
  [[ -n "$root" ]] || return 1
  local envf="$root/apps/admin/.env"
  [[ -f "$envf" ]] || return 1
  acceptance_admin_is_loopback "${ADMIN:-}" || return 1
  local u p
  u=$(grep -E '^COGS_ADMIN_USER=' "$envf" | head -1 | cut -d= -f2- | tr -d '"' | tr -d "'")
  p=$(grep -E '^COGS_ADMIN_PASSWORD=' "$envf" | head -1 | cut -d= -f2- | tr -d '"' | tr -d "'")
  [[ -n "$u" && -n "$p" ]] || return 1
  COGS_ACCEPTANCE_ADMIN_USER=$u
  COGS_ACCEPTANCE_ADMIN_PASSWORD=$p
  export COGS_ACCEPTANCE_ADMIN_USER COGS_ACCEPTANCE_ADMIN_PASSWORD
  return 0
}
