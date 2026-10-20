# Public admin dashboard

The **admin** app (`apps/admin`) is a single public hostname (you choose the subdomain at launch) that serves an embedded SPA and `/api/*` on the same origin. Per-app `/admin` on loopback remains a local debug UI only.

## First-time setup

1. `npx wrangler login` (or set `COGS_CF_API_TOKEN` for DNS).
2. From the repo root:

```bash
cogs admin init
```

Creates the **root** user in `apps/admin/data/admin.db`. There is **no default password** in code or `.env`.

3. Full launch (tunnel + DNS):

```bash
cogs launch admin -y
```

Launch **refuses** to proceed if no users exist. It starts **`cogs supervisor run`** in the background when needed.

## Architecture

- **Browser** → Cloudflare Tunnel → **admin HTTP** (SPA + session cookie).
- **admin HTTP** → unix socket → **supervisor** (metrics, SQL, redeploy only).
- Other apps **cannot** call the supervisor socket (peer executable check + session RBAC).

## RBAC

| Permission | Meaning |
|------------|---------|
| `metrics.read` | Host + app metrics |
| `logs.read` | Reserved for log streaming |
| `sql.read` / `sql.write` | SQL console per app |
| `redeploy` | `git pull --ff-only` + `cogs redeploy <app>` |
| `users.manage` | Create scoped admins (cannot grant root) |

Root bypasses all checks.

## SQL console

The UI sends **database ids** registered from the workspace (each app’s `COGS_DB_PATH`). Raw SQL runs only inside the supervisor executor — not in app handlers.

## DNS / Pages safety (multi-app zones)

CoGS records hostnames it manages in `.cogs/dns-ownership.json` at the workspace root. Before changing a CNAME, **`dnsguard`** blocks the operation if another CoGS app already owns that FQDN with a different target. That protects unrelated Pages apps on the same zone (e.g. `hyperreader.eu`) from being overwritten. Re-deploying the **same** app to the same hostname is allowed.

## Dev notes

- `go run ./cmd/app serve` with `COGS_ADMIN_DEV=1` relaxes unix peer checks when the binary path is not under `apps/admin`.
- Rebuild UI: `cd apps/admin/frontend && npm run build` (output embedded in `internal/ui/dist`).
