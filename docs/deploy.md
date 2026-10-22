# Deploying apps (push / pull)

CoGS deploys **per app**, not as a monolithic “git pull the whole repo and restart everything.” Each app declares how its **UI** and **backend** ship in `apps/<name>/cogs.app.toml`. The **admin** dashboard and **`cogs up`** on the host finish the loop without SSH.

## Canonical app id

The **workspace name** in `cogs.workspace.json` is the app id everywhere that matters: deploy history, dashboard cards, supervisor RPCs.

| App id | Path | UI | Backend deploy |
|--------|------|-----|----------------|
| `todo` | `apps/todo` | **Push** — CoGS builds + Worker static assets | Release swap on host |
| `goldilocks` | `apps/goldilocks` | **Pull** — Cloudflare Workers Builds | Release swap when build succeeds |
| `admin` | `apps/admin` | Embedded SPA (same process as API) | Release swap only; **never** pull mode |

Tunnel and Worker **resource names** can stay longer (`cogs-todo-app`, `cogs-admin`); they are not the app id.

## Source of truth: `cogs.app.toml`

Flat TOML (no tables). Required and common keys:

```toml
name = "todo"
kind = "split"          # split | combined
zone = "example.com"
deploy_mode = "push"    # push | pull (pull forbidden for kind = combined)
deploy_branch = "main"
git_repo = "owner/repo" # for fetch + pull-mode Builds

page_subdomain = "todo"
api_subdomain = "todo-api"
pages_dir = "public"
worker_name = "cogs-todo-app"   # split apps: static-assets Worker
tunnel_name = "cogs-todo-app"
port = 18787
bind_host = "127.0.0.1"
```

**Pull-only:** `root_directory = "apps/goldilocks"` when the monorepo path differs from the app folder.

`cogs launch`, `cogs doctor`, and `cogs up` read this file. Legacy `cogs.launch.json` is still loaded for older apps but must not be rewritten by launch.

## Mental model

```
                    push (todo)
Push to main ──► CoGS builds public/ ──► wrangler deploy (assets-only Worker)
              └──► deployBackend(commit) on host ──► health ──► .cogs/releases/<app>/current

                    pull (goldilocks)
Push to main ──► Workers Builds (npm run build + wrangler deploy)
              └──► Queue event ──► cogs up pulls queue ──► deployBackend(commit) only
              └──► build.failed ──► deploy log only (running API untouched)
```

- **UI hostname** (`todo.<zone>`): Worker custom domain + DNS (not Cloudflare Pages upload for split apps).
- **API hostname** (`todo-api.<zone>`): unchanged — Tunnel → loopback Go process.

**Admin** stays **combined**: one hostname, SPA embedded in the Go binary. A failed Workers Builds frontend must not take down mission control.

## Shared backend: `deployBackend`

One path used by push redeploy, pull queue success, dashboard **Redeploy**, and **Rollback**:

1. `git fetch` (uses host **`gh`** git credentials, or a bearer token when configured for git).
2. Detached **worktree** at `.cogs/worktrees/<app>/<commit>/` — live tree is not reset.
3. `go build` → `.cogs/releases/<app>/<full-sha>/cogs-app` (or `cogs-admin` for combined).
4. Start binary on ephemeral port, `GET /health`, then write **`current`** (and move old `current` to **`previous`**) as **text files** (Windows-safe).
5. Restart supervised process (`cogs up` / pidfile under `apps/<name>/.cogs/run/`).
6. Append **DeployEvent** in `.cogs/deployments.json` (`mode`, `status`, `commit`, errors).

**Rollback** activates `previous` and restarts — same mechanism.

Quick CLI redeploy: `cogs redeploy <app>` = `launch --quick -y`, which rebuilds push UI (when not pull) then runs `deployBackend`.

## Push mode (CoGS ships UI)

From repo root:

```bash
cogs launch todo -y      # first time: tunnel, DNS, Worker, Builds N/A
cogs redeploy todo       # UI build + Worker static assets + backend release
```

Worker config is generated under `apps/todo/.cogs/generated/worker/wrangler.toml` with `[assets] directory` pointing at built `public/` — **no Worker script**, so static requests are not billed as invocations.

## Pull mode (Cloudflare ships UI)

**Goldilocks** uses pull mode. One-time (and after repo changes):

1. Install the **Cloudflare GitHub App** in the dashboard (API cannot do this): Workers & Pages → Builds → Connect.
2. On the **CoGS host** (same user as `cogs up` / admin): `gh auth login && gh auth setup-git` so Builds setup can resolve the repo (see [GitHub access](#github-access-workspace)).
3. `cogs launch goldilocks -y` (flags after the app name work too: `cogs launch goldilocks -y`) — starts the API, deploys the Worker UI once with the real API hostname in `public/config.js`, then: repo connection, Builds trigger (`COGS_PUBLIC_API_URL=… npm run build`, `npx wrangler deploy` from `apps/goldilocks/`), queue **`cogs-deploys`**, **HTTP pull consumer**, and per-worker build event subscription.
4. Queue id is written to **`.cogs/boot/cogs-up.env`** as `COGS_DEPLOY_QUEUE_ID=...` (only supervisor keys — do not put `COGS_LISTEN` there). **`cogs up`** loads whitelisted keys from that file (boot scripts source the same file). Restart `cogs up` after launch if it was already running.

`cogs up` polls the queue over HTTPS (outbound only). Match `workerName` + branch to app; success triggers backend-only deploy. Pull errors log once per distinct failure (check `.cogs/run/up.log`).

**Compatibility:** New UI may hit old API for one deploy window — keep API changes backward compatible; dashboard shows a note on pull-mode apps.

## GitHub access (workspace)

All GitHub access is **host-level** via the [GitHub CLI](https://cli.github.com/) (`gh`). There is no dashboard OAuth app and no `COGS_GITHUB_CLIENT_ID` for admin.

| Need | What to run |
|------|-------------|
| Private repos, `git fetch`, pull-mode Builds repo id | `gh auth login && gh auth setup-git` on the CoGS host |
| One-time host prep (GitHub + Cloudflare) | `scripts/bootstrap-host.sh` |
| Check from shell | `cogs github status` or `gh auth status` |
| Convenience wrapper | `cogs github login` (runs `gh auth login` when `gh` is installed) |
| Paste a PAT (automation) | `COGS_GITHUB_TOKEN=… cogs github login --token` |

**Admin dashboard (root):** **`GET /api/auth/status`** reports GitHub + Cloudflare login from the host; **`GET /api/repos`** lists repos via `gh repo list`; **`POST /api/apps`** clones (or uses the workspace checkout), registers **`cogs.app.toml`**, and starts **`cogs launch -y --no-serve`** in the background. Details: [admin-dashboard.md](admin-dashboard.md).

**Private repos:** log in as a GitHub user who can see the repo (org SSO authorized if required). Run `gh` as the **same OS user** that runs admin and **`cogs up`**.

The **Cloudflare GitHub App** (Workers Builds) is separate — install it in the Cloudflare dashboard; it does not replace host `gh` login.

## Process supervisor: `cogs up` / `cogs down`

```bash
cogs up              # daemon: restart apps + tunnels, poll deploy queue
cogs up --foreground # foreground (Docker compose, systemd, rc.d)
cogs down            # stop apps CoGS started
```

Boot helpers under `.cogs/boot/` (systemd unit, FreeBSD rc.d, launchd plist, Windows scheduled-task cmd, plus `cogs-up.sh` for cron/Docker).

**`cogs up` also keeps `cogs supervisor run` alive** (admin dashboard).

## Public admin dashboard

See [admin-dashboard.md](admin-dashboard.md). Root users: host **`gh`** / **`wrangler`** status, **Add app** (install from repo + launch), per-app deploy mode / commit / status / log tail, **Redeploy** and **Rollback**.

**Admin HTTP:** `/api/auth/status`, `/api/repos`, `/api/apps` (install). **Supervisor** (unix socket): `redeploy`, `rollback`, `deploy_status`, metrics, SQL, logs.

## Where is “prod”?

CoGS does not define a `develop` vs `main` branch in the repo. **Production** is:

- Your **host** (FreeBSD, Linux, Docker, Windows) running **`cogs up`** and app binaries from **`.cogs/releases/<app>/current`**
- **Cloudflare** (Tunnel, Worker UI, optional D1/R2 projections)
- Deploys track **`deploy_branch`** (usually `main`) per app in `cogs.app.toml`

CI/CD is not duplicated inside CoGS: push mode runs builds on the host (or via dashboard redeploy); pull mode runs UI builds on Workers Builds.

## Agent checklist

| Intent | Command |
|--------|---------|
| First-time / infra / zone | `cogs launch <app> -y` |
| Ship code (push app) | `cogs redeploy <app>` or dashboard Redeploy |
| Ship backend only (pull app) | Merge to `deploy_branch`; wait for Builds + queue (or manual redeploy backend) |
| Undo backend | Dashboard Rollback or supervisor `rollback` |
| Fix config / ports / DNS | `cogs doctor`, `cogs tunnel fix --dir apps/<app>` |
| GitHub for fetch / Add app | `gh auth login && gh auth setup-git` (or `scripts/bootstrap-host.sh`) |
| Verify host supervisor | `cogs up --foreground` or `cogs down` then `cogs up` |

When changing deploy behavior in code, **update this file and [AGENTS.md](AGENTS.md)** in the same change.
