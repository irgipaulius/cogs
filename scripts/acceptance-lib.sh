# Shared helpers for acceptance-reconcile.sh (also self-tested).
acceptance_up_log_since_start() {
  local path=$1
  [[ -f "$path" ]] || return 0
  awk 'BEGIN{p=0} /^cogs up start pid=/{buf=$0"\n"; p=1; next} p{buf=buf $0 "\n"} END{printf "%s", buf}' "$path"
}
