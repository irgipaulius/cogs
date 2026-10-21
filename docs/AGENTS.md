# CoGS agent playbook

This document is for **LLM coding agents** operating the [CoGS](https://github.com/irgipaulius/cogs) repo: a **Cloudflare app orchestrator** that runs **Go + SQLite** on the operator’s machine (Mac, NAS, VPS, Docker, FreeBSD, Windows) and uses **Workers (static UI) + Tunnel + D1/R2** at the edge.

Read this before changing launch behavior, adding apps, or deploying remotely.

**Deploy deep dive:** [deploy.md](deploy.md) (push/pull, releases, queue, GitHub).

## Mental model

| Layer | Where it runs | Source of truth |
|--------|----------------|-----------------|
| UI (split apps) | Cloudflare **Worker static assets** (`todo.<zone>`) | Built files in app `public/`; push = CoGS deploys Worker, pull = Workers Builds |
| UI (admin) | Same Go process as API (`cogs-admin.<zone>`) | `apps/admin/frontend` → embedded `internal/ui/dist` |
| API | Loopback on host (`bind_host` + `port` in **`cogs.app.toml`**) | Go binary from **`.cogs/releases/<app>/current`** when set, else `apps/<app>/.cogs/run/` |
| Public API URL | Cloudflare Tunnel → loopback | `<api_subdomain>.<zone>` |
| Edge cache | D1 / R2 (optional) | Projected via `cogs sync`; SQLite wins |

**Do not** treat D1 as authoritative. **Do not** add raw SQL in backend API handlers—use goose + sqlc layout and `database/sql` (engine raw SQL only via project “queries” util if present).

## Repository layout

```
cogs.workspace.json     # monorepo app index (canonical app ids: todo, admin, goldilocks)
apps/<name>/
  cogs.app.toml         # per-app source of truth (deploy_mode, zone, ports, worker_name, …)
  cogs.launch.json      # legacy; do not auto-save from launch
cmd/cogs/               # CLI entry
internal/cli/           # launch, up, deploy, cf, github, queue pull
pkg/deploy, pkg/release # shared deployBackend + current/previous pointers
pkg/appcfg              # cogs.app.toml parser
scripts/install.sh      # curl | sh binary install
```

Discover apps:

```bash
cogs apps list
```

Launch by **workspace app name** (from repo root):

```bash
./cogs launch todo -y          # full provision (first time or infra change)
./cogs redeploy todo           # quick: push UI + Worker assets + deployBackend
./cogs up --foreground         # supervise apps, tunnels, deploy queue (host prod loop)
```

Equivalent paths: `apps/todo`, or `./cogs apps/todo` when config is present.

## CLI install (agent should prefer one)

1. **Release binary** (after tags exist):

   ```bash
   curl -fsSL https://raw.githubusercontent.com/irgipaulius/cogs/main/scripts/install.sh | sh
   export PATH="$HOME/.local/bin:$PATH"
   ```

2. **Clone + build** (always valid in dev):

   ```bash
   git clone https://github.com/irgipaulius/cogs.git && cd cogs
   ./cogs version    # wrapper builds dist/cogs if missing
   ```

Windows: `scripts/install.ps1` via `irm … | iex`.

## GitHub (workspace token)

Many deploy paths **`git fetch`** and pull-mode Builds need a token in **`.cogs/github.json`** (gitignored):

```bash
export COGS_GITHUB_CLIENT_ID=<oauth-app-client-id>
cogs github login              # device flow
# or: COGS_GITHUB_TOKEN=… cogs github login --token
cogs github status
```

Dashboard **Login with GitHub** writes the same file. With **`-y`**, missing token must fail with an explicit `cogs github login` message — never hang on prompts.

## Reference app: `apps/todo` (push)

**`cogs.app.toml`:** `name = "todo"`, `deploy_mode = "push"`, `worker_name`, `git_repo`, `deploy_branch`.

**Required env (launch writes `.env`):**

- `COGS_SESSION_SECRET` — random string
- `COGS_PUBLIC_PAGE_ORIGIN` — `https://todo.<zone>`
- `COGS_PUBLIC_API_URL` — `https://todo-api.<zone>`
- **Workspace DNS token** — `.cogs/cloudflare/dns.token` (or zone file + `tokens.json`); one token reused by all apps. Optional override: `COGS_CF_API_TOKEN` env.

**Observability:**

- `COGS_OBS=file` (default) or `off` | `dashboard`
- Local admin hub: `http://<COGS_LISTEN>/admin` (loopback Host only; returns 404 on public tunnel hostname)

**Tokens split (important):**

- **Wrangler OAuth** — tunnel / Worker deploy / D1/R2 via `npx wrangler`
- **`COGS_CF_API_TOKEN`** — zone DNS records for UI + API hostnames

If launch fails with tunnel/DNS errors, run from app dir:

```bash
cogs tunnel fix --dir apps/todo
```

## Reference app: `apps/goldilocks` (pull)

**`deploy_mode = "pull"`** — UI from **Workers Builds**; host only runs **`deployBackend`** on queue `build.succeeded`. Requires Cloudflare GitHub App (manual), queue subscription, and **`COGS_DEPLOY_QUEUE_ID`** on the `cogs up` host. See [deploy.md](deploy.md).

Failed frontend builds → deploy log + dashboard error; **do not** replace running backend.

## Launch vs redeploy (agent decision tree)

```
Need new tunnel/DNS/Worker trigger/pull setup or zone change?
  yes → cogs launch <app> -y
  no  → cogs redeploy <app>     # push apps: UI + backend release
```

`redeploy` = `launch --quick -y` (skips Cloudflare provisioning).

- After **frontend** only (push app): `redeploy` is enough.
- After **`cogs.app.toml`** tunnel/worker/mode changes: full `launch`.
- **Pull app** backend-only ship: merge to `deploy_branch` + queue, or dashboard Redeploy (backend).
- **Admin**: combined app — redeploy rebuilds embedded UI + backend release; never `deploy_mode = pull`.

Backend deploy is always **worktree + release pointer**, not `git pull` + dirty tree.

## Adding a new app

**From dashboard (root):** GitHub login → Add app (repo, mode, subdomain) → scaffold + `go work use` + launch.

**Manual:**

1. Create `apps/<name>/` with **`cogs.app.toml`**, `cmd/app`, migrations, `public/` or frontend.
2. Register in root `cogs.workspace.json` and `go work use ./apps/<name>`.
3. `cogs launch <name> -y` once.

Use `cogs new` for greenfield outside `apps/`; for this monorepo, copy `apps/todo` structure.

## Remote server (NAS / Linux / Docker) — prod loop

**Production** = host running **`cogs up`** + releases under `.cogs/releases/` + tunnel, not a special git branch in this repo.

1. Clone repo on server; copy app `.env` securely; never commit `.env`.
2. `cogs github login` if deploys must fetch private repos.
3. **`cogs up --foreground`** (or systemd/rc.d/launchd from `.cogs/boot/` after `cogs up` once).
4. Pull-mode: set **`COGS_DEPLOY_QUEUE_ID`** on that host.
5. **Tunnel handoff:** `cogs deploy` / launch with tunnel; `COGS_RUNNER_ID` + lease watch; separate prod DB — **`migrate` on prod only**.
6. Verify: `curl -sS https://todo-api.<zone>/health`, UI on `https://todo.<zone>`.
7. **Public ops dashboard:** `cogs admin init` → `cogs launch admin -y` → `https://cogs-admin.<zone>`. Per-app `/admin` stays loopback-only.

If SSH details are unknown, **stop and ask the user** for host, user, arch, and tunnel placement.

## Ports and workspace deploy

- **`cogs.workspace.json` → `deploy`**: `base_port` (default **18787**), `port_step`, `reserved_ports`.
- Ports live in **`cogs.app.toml`** per app (todo 18787, admin 18786, goldilocks 18788).
- **`cogs deploy`** at repo root: all apps, default quick + parallel. Flags: `--app NAME`, `--ping`, `--dry-run`, etc.
- **Agents**: prefer **`--json`** and **`--dry-run`**; global **`-y`** for non-interactive.

## Commands cheat sheet

| Intent | Command |
|--------|---------|
| List apps | `cogs apps list` |
| Deploy entire workspace | `cogs deploy --ping` |
| First-time / infra | `cogs launch todo -y` |
| Daily ship (push) | `cogs redeploy todo` |
| Host supervisor | `cogs up` / `cogs down` |
| GitHub token | `cogs github login` / `status` |
| Health check | `cogs ping todo` |
| DNS/tunnel repair | `cogs tunnel fix --dir apps/todo` |
| Doctor | `cogs doctor` (workspace + apps) |
| Build cross-compile | `cogs build --target linux/arm64` |
| Sync SQLite → D1/R2 | `cd apps/todo && cogs sync -env .env` |
| Admin rollback / redeploy | Dashboard or supervisor (see admin-dashboard.md) |

## What agents should not do

- Run **Prisma migrate** unless the user explicitly says **"please migrate prisma"** (CoGS uses goose).
- Commit `.env`, API tokens, `.cogs/github.json`, or wrangler session secrets.
- Force-push `main` or skip git hooks unless asked.
- Assume `/admin` works on the public API hostname—it is **loopback-only** by design.
- Move **admin** UI to Workers Builds (combined app must stay on Go process).
- Edit the attached plan file `deploy_modes_dashboard_*.plan.md` unless the user asks.

## Docs map

| File | Audience |
|------|----------|
| [AGENTS.md](AGENTS.md) | LLM agents (this file) |
| [deploy.md](deploy.md) | Push/pull, releases, queue, GitHub |
| [admin-dashboard.md](admin-dashboard.md) | Public admin SPA + supervisor |
| [quickstart.md](quickstart.md) | Humans, first run |
| [cloudflare-setup.md](cloudflare-setup.md) | CF dashboard + tokens |
| [docker.md](docker.md) | Optional API containers |
| [observe.md](observe.md) | COGS_OBS, admin hub |
| [ingress.md](ingress.md) | tunnel-first vs per-route direct |
| [install.md](install.md) | curl \| sh vs clone |

When updating deploy or launch behavior, **update [deploy.md](deploy.md) and this playbook** in the same PR as CLI changes.
