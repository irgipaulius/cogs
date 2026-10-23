# Public admin dashboard

The **admin** app (`apps/admin`, workspace id **`admin`**) is a single public hostname (`cogs-admin.<zone>` by default) that serves an embedded SPA and `/api/*` on the same origin. Per-app `/admin` on loopback remains a local debug UI only.

Admin is **`kind = combined`** and **`deploy_mode = push` only** — the UI is not deployed via Workers Builds, so a broken frontend build cannot remove mission control.

## First-time setup

1. On the **server** that will run admin and **`cogs up`**, install and log in CLI tools (once per user):

   ```bash
   gh auth login && gh auth setup-git
   wrangler login
   # or: scripts/bootstrap-host.sh
   ```

   Admin runs **`gh`** and **`wrangler whoami`** as this OS user — not via the browser session. DNS can use **`COGS_CF_API_TOKEN`** instead of Wrangler login.

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
- **admin HTTP** → install/repos/auth on the admin process; unix socket → **supervisor** (metrics, SQL, redeploy/rollback).
- Other apps **cannot** call the supervisor socket (peer executable check + session RBAC).

## Deploy operations (dashboard)

| UI | Supervisor op | Effect |
|----|----------------|--------|
| **Redeploy** | `redeploy` | Push apps: quick launch (UI + Worker when split) + **`deployBackend`**. Pull apps: backend release only. Admin: embedded UI build + backend release. |
| **Rollback** | `rollback` | Swap to **previous** release under `.cogs/releases/<app>/` and restart. |
| Hosting tab | (cockpit data) | `deploy_mode`, commit, `last_status`, `last_error`, recent deploy events |

Deploy history lives in **`.cogs/deployments.json`** at workspace root (shared event format for push and pull). App lifecycle state (desired present, releases, reconcile runs) lives in **`.cogs/state.db`** (SQLite).

## Install, reconcile, remove (root only)

| Action | API | Effect |
|--------|-----|--------|
| **Add app** | `POST /api/apps` | `SetDesired(present=true)` + background reconcile |
| **Reconcile** | `POST /api/apps/{name}/reconcile` | Re-run idempotent pipeline |
| **Remove** | `POST /api/apps/{name}/remove` | `present=false` + GC reconcile |
| **Progress** | `GET /api/reconcile/stream` (SSE) | `event: apps` — JSON array of app reconcile snapshots |
| **Log** | `GET /api/apps/{name}/reconcile/log` | Latest reconcile log file |

There is **no** finish-install or install-job API.

## GitHub and Add app (root only)

Host-level CLI auth only — **no** dashboard “Sign in with GitHub” and **no** `COGS_GITHUB_CLIENT_ID` on the admin server.

### Host prerequisites

| Tool | Used for |
|------|-----------|
| **`gh`** | Repo list, clone, CoGS config hints (`has_cogs_config` on `/api/repos`) |
| **`wrangler`** | Shown on status strip; **pull** mode needs it during background launch (Workers Builds setup) |

Run both as the user that starts **admin** and **`cogs up`**. Private repos: the logged-in GitHub account must have access (collaborator / org member; authorize org SSO if required).

### Admin HTTP API (root session)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/auth/status` | `{ github, cloudflare }` from host CLIs |
| GET | `/api/repos` | Cached `gh repo list` (+ optional `?q=` filter) |
| POST | `/api/apps` | Install app from repo (see body below) |

**`POST /api/apps`** JSON:

- **`repo`** (required): `owner/name`
- **`name`** (optional): workspace app id; defaults to `name` in `cogs.app.toml`
- **`deploy_mode`**: `push` or `pull` (updates config when registering)
- **`page_subdomain`**: used only when scaffolding an empty app (`scaffold_only` — normal install reads the repo’s config)
- **`scaffold_only`**: if true, create empty `apps/<name>/` template instead of cloning (advanced)

**Install flow (default):**

1. **`POST /api/apps`** writes desired state to **`.cogs/state.db`** and enqueues **reconcile**.
2. Reconcile stages: source → validate → build workspace → secrets → cf_resources → tunnel → worker → dns → build release → migrate → activate → GC.
3. Watch progress on **Apps** via **`GET /api/reconcile/stream`** (SSE).

CLI equivalent: register app per [deploy.md](deploy.md), then **`cogs launch <app> -y`** or trigger **`POST /api/apps/{name}/reconcile`**.

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
