# CoGS agent playbook

This document is for **LLM coding agents** operating the [CoGS](https://github.com/irgipaulius/cogs) repo: a **Cloudflare app orchestrator** that runs **Go + SQLite** on the operator’s machine (Mac, NAS, VPS) and uses **Pages + Tunnel + D1/R2** at the edge.

Read this before changing launch behavior, adding apps, or deploying remotely.

## Mental model

| Layer | Where it runs | Source of truth |
|--------|----------------|-----------------|
| UI | Cloudflare Pages (`todo.<zone>`) | Built static assets in app `public/` |
| API | Loopback on host (`deploy.bind_host:port` from `cogs.workspace.json`) | Go binary + **SQLite file on disk** |
| Public API URL | Cloudflare Tunnel → same loopback | `todo-api.<zone>` CNAME → tunnel |
| Edge cache | D1 / R2 (optional) | Projected via `cogs sync`; SQLite wins |

**Do not** treat D1 as authoritative. **Do not** add raw SQL in backend API handlers—use Prisma-style patterns only if the project already has them; in CoGS apps use **goose + sqlc layout** and `database/sql`.

## Repository layout

```
cogs.workspace.json     # monorepo app index (orchestrator)
apps/<name>/            # deployable apps (each has cogs.launch.json)
cmd/cogs/               # CLI entry
internal/cli/           # launch, tunnel, deploy, cf wrappers
pkg/                    # framework libraries (server, observe, launch, workspace)
scripts/install.sh      # curl | sh binary install
examples/               # smaller demos (geo-points, worker-client)—not primary apps
```

Discover apps:

```bash
cogs apps list
```

Launch by **workspace app name** (from repo root):

```bash
./cogs launch todo -y          # full provision (first time or infra change)
./cogs redeploy todo           # fast: UI build, Pages, migrate, serve
```

Equivalent paths: `apps/todo`, or `./cogs apps/todo` when `cogs.launch.json` is present.

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

## Reference app: `apps/todo`

Use this to validate launch/redeploy/tunnel/DNS.

**Required env (launch writes `.env`):**

- `COGS_SESSION_SECRET` — random string
- `COGS_PUBLIC_PAGE_ORIGIN` — `https://todo.<zone>`
- `COGS_PUBLIC_API_URL` — `https://todo-api.<zone>`
- `COGS_CF_API_TOKEN` — Cloudflare API token with **Zone DNS Edit** for the apex zone (DNS automation; Wrangler OAuth alone is insufficient for CNAME creation)

**Observability:**

- `COGS_OBS=file` (default) or `off` | `dashboard`
- Local admin hub: `http://<COGS_LISTEN>/admin` (loopback Host only; returns 404 on public tunnel hostname)

**Tokens split (important):**

- **Wrangler OAuth** — tunnel/Pages/D1/R2 API via `npx wrangler`
- **`COGS_CF_API_TOKEN`** — zone DNS records for `todo` / `todo-api` hostnames

If launch fails with tunnel/DNS errors, run from app dir:

```bash
cogs tunnel fix --dir apps/todo
```

## Launch vs redeploy (agent decision tree)

```
Need new D1/R2/tunnel/DNS/Pages project or zone change?
  yes → cogs launch <app> -y
  no  → cogs redeploy <app>
```

`redeploy` = `launch --quick -y` (skips Cloudflare provisioning).

After **frontend** changes only: `redeploy` is enough.

After **cogs.launch.json** or tunnel name changes: full `launch`.

## Adding a new app to the workspace

1. Create `apps/<name>/` with `cogs.launch.json`, `cmd/app`, migrations, optional `frontend/`.
2. Register in root `cogs.workspace.json`:

   ```json
   { "name": "myapp", "path": "apps/myapp", "description": "…" }
   ```

3. Run `cogs launch <name> -y` once from repo root.

Use `cogs new` for greenfield outside `apps/` if the user wants a standalone repo; for this monorepo, copy `apps/todo` structure.

## Remote server (NAS / Linux) deployment — planned path

Goal: same git repo on server, **linux/arm64** or **linux/amd64** binary, systemd service, tunnel runner on server instead of Mac.

**Agent steps (manual until `cogs deploy` is fully wired for your host):**

1. On server: clone repo, install Go or use `cogs build --target linux/arm64` from dev machine and copy binary.
2. Copy app `.env` securely (contains secrets); never commit `.env`.
3. `cd apps/todo && /path/to/cogs migrate -env .env` then `serve` under systemd (see `cogs service render systemd`).
4. **Tunnel handoff (automatic):** prod runs `cogs deploy` (or `launch`/`redeploy` with tunnel). New machine claims via `COGS_RUNNER_ID` + `/.well-known/cogs-runner`, Cloudflare API drops old tunnel connectors, then starts local tunnel+API. Other hosts run **lease watch** (auto on deploy) and **yield** when they see a newer claim. Separate prod DB: `migrate` on prod only.
5. Verify: `curl -sS https://todo-api.<zone>/health`, Pages UI loads. **Public ops dashboard**: `cogs admin init` then `cogs launch admin -y` → `https://admin.<zone>`. Per-app `/admin` stays loopback-only.

If SSH details are unknown, **stop and ask the user** for host, user, arch, and whether cloudflared should move off the Mac.

## Ports and workspace deploy

- **`cogs.workspace.json` → `deploy`**: `base_port` (default **18787**), `port_step`, `reserved_ports` (includes **8080**).
- Each app: `cogs.launch.json` **`port`: 0** = auto-assign; app *n* gets `base_port + n*step`, skipping reserved/used ports.
- **`cogs deploy`** at repo root: all apps, default **quick + parallel + background serve**. Flags: `--app NAME`, `--ping`, `--dry-run`, `--serial`, `--full`, `--skip-ui-build`, etc.
- **Agents**: prefer **`--json`** and **`--dry-run`**; global flags work on every subcommand (`COGS_JSON=1` also enables JSON).

## Commands cheat sheet

| Intent | Command |
|--------|---------|
| List apps + ports | `cogs apps list` |
| Deploy entire workspace | `cogs deploy --ping` |
| First-time / infra | `cogs launch todo -y` |
| Daily deploy | `cogs redeploy todo` |
| Health check | `cogs ping todo` |
| DNS/tunnel repair | `cogs tunnel fix --dir apps/todo` |
| Doctor | `cogs doctor -env apps/todo/.env` |
| Build cross-compile | `cogs build --target linux/arm64` |
| Sync SQLite → D1/R2 | `cd apps/todo && cogs sync -env .env` |

## What agents should not do

- Run **Prisma migrate** commands unless the user explicitly says **"please migrate prisma"** (CoGS uses goose; rule inherited from user global prefs).
- Commit `.env`, API tokens, or `wrangler` session secrets.
- Force-push `main` or skip git hooks unless asked.
- Assume `/admin` works on the public API hostname—it is **loopback-only** by design.

## Docs map

| File | Audience |
|------|----------|
| [AGENTS.md](AGENTS.md) | LLM agents (this file) |
| [quickstart.md](quickstart.md) | Humans, first run |
| [cloudflare-setup.md](cloudflare-setup.md) | CF dashboard + tokens |
| [observe.md](observe.md) | COGS_OBS, admin hub |
| [ingress.md](ingress.md) | tunnel-first vs per-route direct |
| [install.md](install.md) | curl \| sh vs clone |

When updating behavior, **update this playbook** in the same PR as CLI changes.
