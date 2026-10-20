#!/usr/bin/env sh
# Install cogs CLI from GitHub releases (linux/darwin/freebsd, amd64/arm64).
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/irgipaulius/cogs/main/scripts/install.sh | sh
#   COGS_VERSION=v0.1.0 sh install.sh
#   COGS_INSTALL_DIR=/usr/local/bin COGS_VERSION=latest sh install.sh
set -eu

REPO="${COGS_GITHUB_REPO:-irgipaulius/cogs}"
INSTALL_DIR="${COGS_INSTALL_DIR:-${HOME}/.local/bin}"
VERSION="${COGS_VERSION:-latest}"

die() {
  echo "cogs install: $*" >&2
  exit 1
}

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  linux) goos=linux ;;
  darwin) goos=darwin ;;
  freebsd) goos=freebsd ;;
  *) die "unsupported OS: $os" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64) goarch=amd64 ;;
  aarch64|arm64) goarch=arm64 ;;
  *) die "unsupported arch: $arch" ;;
esac

if ! command -v curl >/dev/null 2>&1; then
  die "curl is required"
fi

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)"
  [ -n "$VERSION" ] || die "could not resolve latest release for ${REPO}"
fi

tag="$VERSION"
case "$tag" in
  v*) ;;
  *) tag="v${tag}" ;;
esac

asset="cogs_${tag#v}_${goos}_${goarch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${tag}/${asset}"

tmpdir="$(mktemp -d 2>/dev/null || mktemp -d -t cogs)"
trap 'rm -rf "$tmpdir"' EXIT INT HUP

echo "Downloading ${url} …"
curl -fsSL "$url" -o "${tmpdir}/archive.tar.gz" || die "download failed (is ${tag} published for ${goos}/${goarch}?)"

tar -xzf "${tmpdir}/archive.tar.gz" -C "$tmpdir"
bin="${tmpdir}/cogs"
[ -x "$bin" ] || die "archive did not contain cogs binary"

mkdir -p "$INSTALL_DIR"
install -m 755 "$bin" "${INSTALL_DIR}/cogs"

echo "Installed cogs to ${INSTALL_DIR}/cogs (${tag}, ${goos}/${goarch})"
if ! echo ":${PATH}:" | grep -q ":${INSTALL_DIR}:"; then
  echo "Add to PATH: export PATH=\"${INSTALL_DIR}:\$PATH\""
fi
