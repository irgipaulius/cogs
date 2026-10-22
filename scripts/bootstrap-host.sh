#!/usr/bin/env bash
# One-time host setup for CoGS: GitHub CLI + Cloudflare Wrangler login.
set -euo pipefail

echo "=== CoGS host bootstrap ==="
echo "GitHub (repo access for admin + pull-mode builds)"
if command -v gh >/dev/null 2>&1; then
  gh auth status --hostname github.com 2>/dev/null || gh auth login --hostname github.com --git-protocol https --skip-ssh-key
  gh auth setup-git
else
  echo "Install GitHub CLI: https://cli.github.com/"
  exit 1
fi

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
echo "Done. Admin dashboard reads auth from host gh + wrangler (no COGS_GITHUB_CLIENT_ID)."
