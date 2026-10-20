#!/usr/bin/env bash
set -euo pipefail
TARGET="${1:-$(go env GOOS)/$(go env GOARCH)}"
GOOS="${TARGET%%/*}"
GOARCH="${TARGET##*/}"
OUT="dist/deps/${GOOS}_${GOARCH}"
mkdir -p "$OUT"
echo "CoGS deps: building cloudflared for ${GOOS}/${GOARCH} (see cloudflared docs if this fails)"
if command -v go >/dev/null; then
  (cd /tmp && [ -d cloudflared-src ] || git clone --depth 1 https://github.com/cloudflare/cloudflared.git cloudflared-src)
  (cd /tmp/cloudflared-src && CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" go build -o "$OLDPWD/$OUT/cloudflared" ./cmd/cloudflared)
  echo "wrote $OUT/cloudflared"
else
  echo "go not found; download cloudflared manually for $TARGET"
fi
echo "Litestream: use built-in 'app backup' or install litestream separately (optional)."
