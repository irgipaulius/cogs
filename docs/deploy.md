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

1. `git fetch` (uses workspace `.cogs/github.json` token when set).
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
2. `cogs github login` (or dashboard **Login with GitHub**) — token in `.cogs/github.json`.
3. `cogs launch goldilocks -y` (flags after the app name work too: `cogs launch goldilocks -y`) — starts the API, deploys the Worker UI once with the real API hostname in `public/config.js`, then: repo connection, Builds trigger (`COGS_PUBLIC_API_URL=… npm run build`, `npx wrangler deploy` from `apps/goldilocks/`), queue **`cogs-deploys`**, **HTTP pull consumer**, and per-worker build event subscription.
4. Queue id is written to **`.cogs/boot/cogs-up.env`** as `COGS_DEPLOY_QUEUE_ID=...` (only supervisor keys — do not put `COGS_LISTEN` there). **`cogs up`** loads whitelisted keys from that file (boot scripts source the same file). Restart `cogs up` after launch if it was already running.

`cogs up` polls the queue over HTTPS (outbound only). Match `workerName` + branch to app; success triggers backend-only deploy. Pull errors log once per distinct failure (check `.cogs/run/up.log`).

### Migrating UI from Pages to Worker

Split apps no longer use Pages. Before the Worker custom domain will attach:

1. In Cloudflare **Pages**, remove the **custom domain** for the UI hostname (e.g. `todo.<zone>`). Pages ownership blocks the same name on Workers.
2. Remove any **CNAME** pointing the UI hostname at `*.pages.dev`.
3. Run `cogs redeploy <app>` (push) or `cogs launch <app> -y` so wrangler deploys static assets with `[[routes]]` + `custom_domain = true` for the UI host.
4. API hostname stays on the tunnel only (`todo-api.<zone>`).

**Compatibility:** New UI may hit old API for one deploy window — keep API changes backward compatible; dashboard shows a note on pull-mode apps.

## GitHub login (workspace)

| Surface | Command / action |
|---------|------------------|
| CLI | `cogs github login` (device flow), `cogs github login --token`, `COGS_GITHUB_TOKEN`, `cogs github status` |
| Dashboard | **Login with GitHub** (same token file) |
| Non-interactive | `-y` without token → exit with message to run `cogs github login` |

Requires env **`COGS_GITHUB_CLIENT_ID`** (public OAuth app client id). Used for `git fetch`, repo list in **Add app**, and pull-mode Builds setup.

Cloudflare GitHub App is **separate** from GitHub OAuth above.

## Process supervisor: `cogs up` / `cogs down`

```bash
cogs up              # daemon: restart apps + tunnels, poll deploy queue
cogs up --foreground # foreground (Docker compose, systemd, rc.d)
cogs down            # stop apps CoGS started
```

Boot helpers under `.cogs/boot/` (systemd unit, FreeBSD rc.d, launchd plist, Windows scheduled-task cmd, plus `cogs-up.sh` for cron/Docker).

**`cogs up` also keeps `cogs supervisor run` alive** (admin dashboard). Do not run a legacy per-app watchdog and `cogs up` against the same pidfiles — pick one supervisor. See [freebsd.md](os-guides/freebsd.md).

## Public admin dashboard

See [admin-dashboard.md](admin-dashboard.md). Root users: GitHub login, **Add app** (scaffold + `go work use` + launch), per-app deploy mode / commit / status / log tail, **Redeploy** and **Rollback**.

Supervisor ops (unix socket): `redeploy`, `rollback`, `deploy_status`, `github_login_start`, `github_login_poll`, `github_repos`, `add_app`.

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
| GitHub for fetch / Add app | `cogs github login` |
| Verify host supervisor | `cogs up --foreground` or `cogs down` then `cogs up` |

When changing deploy behavior in code, **update this file and [AGENTS.md](AGENTS.md)** in the same change.
