#!/usr/bin/env bash
# One-time host setup for CoGS: install gh, log in gh + wrangler as the CoGS service user.
set -euo pipefail

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
install_gh() {
  if command -v gh >/dev/null 2>&1; then
    return 0
  fi
  case "$OS" in
    freebsd) sudo pkg install -y gh ;;
    darwin) brew install gh ;;
    linux)
      if command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update && sudo apt-get install -y gh
      else
        echo "Install gh: https://cli.github.com/" >&2
        exit 1
      fi
      ;;
    *) echo "Install gh: https://cli.github.com/" >&2; exit 1 ;;
  esac
}

echo "=== CoGS host bootstrap ==="
install_gh

echo "GitHub (repo access for admin + pull-mode builds)"
if gh auth status --hostname github.com 2>/dev/null; then
  echo "gh: already logged in"
else
  gh auth login --hostname github.com --git-protocol https --skip-ssh-key
fi
gh auth setup-git

echo ""
echo "Cloudflare (Workers, tunnels, DNS)"
if command -v wrangler >/dev/null 2>&1; then
  wrangler whoami 2>/dev/null || wrangler login
elif command -v npx >/dev/null 2>&1; then
  npx wrangler whoami 2>/dev/null || npx wrangler login
else
  echo "Install wrangler: npm i -g wrangler"
  exit 1
fi

echo ""
echo "Tip: set COGS_HOME in .cogs/boot/cogs-up.env when cogs runs as a dedicated user (e.g. COGS_HOME=/home/cogs)."
echo "Done. Admin dashboard reads auth from host gh + wrangler."
