#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
grep -q '^install_jq()' "$SCRIPT_DIR/bootstrap-host.sh" || { echo "bootstrap missing install_jq"; exit 1; }
grep -q 'install_jq' "$SCRIPT_DIR/bootstrap-host.sh" || { echo "bootstrap must call install_jq"; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq required for bootstrap-selftest on this host"; exit 1; }
echo "bootstrap-selftest ok"
