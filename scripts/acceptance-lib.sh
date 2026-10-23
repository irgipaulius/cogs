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
