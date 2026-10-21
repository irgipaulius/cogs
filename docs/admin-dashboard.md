# Public admin dashboard

The **admin** app (`apps/admin`, workspace id **`admin`**) is a single public hostname (`cogs-admin.<zone>` by default) that serves an embedded SPA and `/api/*` on the same origin. Per-app `/admin` on loopback remains a local debug UI only.

Admin is **`kind = combined`** and **`deploy_mode = push` only** — the UI is not deployed via Workers Builds, so a broken frontend build cannot remove mission control.

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

4. Keep the host supervised:

```bash
cogs up    # or cogs up --foreground in Docker
```

## Architecture

- **Browser** → Cloudflare Tunnel → **admin HTTP** (SPA + session cookie).
- **admin HTTP** → unix socket → **supervisor** (metrics, SQL, deploy ops, GitHub for root).
- Other apps **cannot** call the supervisor socket (peer executable check + session RBAC).

## Deploy operations (dashboard)

| UI | Supervisor op | Effect |
|----|----------------|--------|
| **Redeploy** | `redeploy` | Push apps: quick launch (UI + Worker when split) + **`deployBackend`**. Pull apps: backend release only. Admin: embedded UI build + backend release. |
| **Rollback** | `rollback` | Swap to **previous** release under `.cogs/releases/<app>/` and restart. |
| Hosting tab | (cockpit data) | `deploy_mode`, commit, `last_status`, `last_error`, recent deploy events |

Deploy history lives in **`.cogs/deployments.json`** at workspace root (shared event format for push and pull).

## GitHub and Add app (root only)

1. Set **`COGS_GITHUB_CLIENT_ID`** on the host running admin (OAuth app client id, public).
2. Dashboard **Login with GitHub** — same token as `cogs github login` → `.cogs/github.json`.
3. **Add app**: pick repo, **push** or **pull** mode, UI subdomain → creates `apps/<name>/cogs.app.toml`, updates `cogs.workspace.json` and `go work use`, runs non-interactive launch.

CLI equivalent: `cogs github login`, then scaffold manually per [deploy.md](deploy.md).

## RBAC

| Permission | Meaning |
|------------|---------|
| `metrics.read` | Host + app metrics |
| `logs.read` | App logs via supervisor |
| `sql.read` / `sql.write` | SQL console per app |
| `redeploy` | Redeploy + rollback for granted apps |
| `users.manage` | Create scoped admins (cannot grant root) |

Root bypasses all checks.

## SQL console

The UI sends **database ids** registered from the workspace (each app’s `COGS_DB_PATH`). Raw SQL runs only inside the supervisor executor — not in app handlers.

## DNS / multi-app zones

CoGS records hostnames in `.cogs/dns-ownership.json`. **`dnsguard`** blocks overwriting another app’s CNAME. Split-app UI hostnames target **Workers** (not Pages) after the deploy-mode update; API hostnames stay on the tunnel.

## Dev notes

- `go run ./cmd/app serve` with `COGS_ADMIN_DEV=1` relaxes unix peer checks when the binary path is not under `apps/admin`.
- Rebuild UI: `cd apps/admin/frontend && npm run build` (output in `internal/ui/dist`).

See also [deploy.md](deploy.md) for push/pull and queue setup.
