# CoGS

**Run your app on your own machine. Let Cloudflare handle the internet.**

CoGS (**C**loudflare + **G**o + **S**QLite) is a small toolkit that wires together the pieces most side projects need: a fast Go API, a single SQLite database you own, a static frontend on **Cloudflare Workers** (assets-only for split apps), and a **Tunnel** so you never open port 8080 on your router. One command provisions the edge; **`cogs redeploy`** or the **admin dashboard** ships changes with release-based backend swaps—no SSH.

No Kubernetes. No monthly database bill for the source of truth. Just a binary, a `.db` file, and Cloudflare doing TLS, DNS, and bot filtering at the edge.

---

## What is this for?

You want a real web app—todo list, internal tool, hobby API—not a serverless demo that forgets your data when you hit a limit.

CoGS is for you if:

- You’re happy running a **Go program on your Mac, NAS, or VPS**
- You want **Cloudflare** for the public UI and HTTPS without managing certificates
- You want **SQLite on disk** as the authoritative database (optional sync to D1/R2 for reads at the edge)
- You’d rather **`cogs launch`** than click through five dashboards every time

It ships with a full **todo app** you can clone, launch, and customize.

---

## How it works (plain English)

Think of three layers:

```
  Browser  ──►  todo.yourdomain.com     (Worker static assets — built HTML/JS)
       │
       └──►  todo-api.yourdomain.com  (Cloudflare Tunnel — no open ports at home)
                    │
                    ▼
              Go API + SQLite on YOUR computer (or NAS), supervised by cogs up
```

1. **Workers (assets only)** serve the UI from `public/` after you build the frontend (**push** mode), or **Workers Builds** builds the UI in the cloud (**pull** mode — host deploys API only when the build succeeds).
2. **Tunnel** connects Cloudflare to your API on loopback—only outbound traffic from your machine.
3. **SQLite** holds the data. You back it up like any file. Optionally, **`cogs sync`** pushes read copies to D1/R2.

Each app declares **push vs pull** in **`apps/<name>/cogs.app.toml`**. Backends deploy via a shared **release** path (worktree build, health check, `.cogs/releases/<app>/current`). Details: [docs/deploy.md](docs/deploy.md).

The **`cogs` CLI** is the orchestrator: Wrangler, migrations, env files, tunnel lifecycle, **`cogs up`**, and host **`gh`** access for `git fetch`, pull-mode Builds, and dashboard **Add app**.

This repo is a **workspace**: several apps can live under `apps/`, listed in `cogs.workspace.json`.

---

## Try it in five minutes

**Prerequisites:** [Go 1.23+](https://go.dev/dl/), [Node.js](https://nodejs.org/) (for the todo UI), a [Cloudflare account](https://dash.cloudflare.com/) with a domain on it, and [Wrangler](https://developers.cloudflare.com/workers/wrangler/) (CoGS runs it via `npx`).

```bash
git clone https://github.com/irgipaulius/cogs.git
cd cogs
./cogs apps list
```

**First time** (tunnel, DNS, Worker UI, D1/R2, builds UI, starts API):

```bash
./cogs launch todo -y
```

You’ll sign in with Wrangler and paste a Cloudflare API token with **Zone → DNS → Edit** so CoGS can create `todo` and `todo-api` hostnames. DNS tokens live under `.cogs/cloudflare/` at workspace level (never commit secrets).

**Every day after** (UI build, Worker deploy, backend release—skips reprovisioning):

```bash
./cogs redeploy todo
```

Optional: run **`cogs up`** on the host so apps, tunnels, and pull-mode deploy queues stay supervised across reboots.

Open the URLs printed at the end. Local admin metrics (loopback only): `http://127.0.0.1:8080/admin`.

More detail: [Quickstart](docs/quickstart.md) · [Deploy (push/pull)](docs/deploy.md) · [Cloudflare setup](docs/cloudflare-setup.md) · [Install CLI without cloning](docs/install.md)

---

## Start your own app

Scaffold a new project, then run it locally before you touch Cloudflare:

```bash
./cogs new myapp --module github.com/YOU/myapp
cd myapp
cp .env.example .env    # set COGS_SESSION_SECRET
go run ./cmd/app serve -env .env
```

Add **`cogs.app.toml`**, register the app in `cogs.workspace.json`, `go work use ./apps/myapp`, and run `./cogs launch myapp -y` when you’re ready to go public—or use the **admin dashboard** (**Add app**) after **`gh auth login`** on the server ([admin-dashboard.md](docs/admin-dashboard.md)).

---

## What you get

| Piece | Role |
|--------|------|
| **Pure Go SQLite** | No CGO; easy cross-compile to Linux ARM (NAS) and amd64 |
| **`cogs launch` / `redeploy` / `up`** | Edge setup vs ship changes vs host supervisor |
| **Push / pull deploy** | Per-app UI on CoGS or Workers Builds — [deploy.md](docs/deploy.md) |
| **Admin dashboard** | Redeploy, rollback, install apps from GitHub (host `gh`) — [admin-dashboard.md](docs/admin-dashboard.md) |
| **Migrations** | Goose + sqlc-friendly layout |
| **Security helpers** | Turnstile sessions, rate limits, optional Cloudflare Access |
| **`cogs sync`** | SQLite stays truth; D1/R2 get projections for edge reads |
| **Reference app** | [apps/todo](apps/todo) — offline queue, R2 uploads, link stats |
| **Examples** | [geo-points](examples/geo-points), [worker-client](examples/worker-client) |

**AI coding agents:** playbook at [docs/AGENTS.md](docs/AGENTS.md).

---

## Install the CLI (optional)

After [releases](https://github.com/irgipaulius/cogs/releases) exist:

```bash
curl -fsSL https://raw.githubusercontent.com/irgipaulius/cogs/main/scripts/install.sh | sh
```

Or keep using `./cogs` from the repo—it builds `dist/cogs` automatically.

---

## License

MIT — see [LICENSE](LICENSE). Use it, fork it, ship something fun.
