# CoGS

**Run your app on your own machine. Let Cloudflare handle the internet.**

CoGS (**C**loudflare + **G**o + **S**QLite) is a small toolkit that wires together the pieces most side projects need: a fast Go API, a single SQLite database you own, a static frontend on **Cloudflare Pages**, and a **Tunnel** so you never open port 8080 on your router. One command provisions the edge; another redeploys when you change code.

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
  Browser  ──►  todo.yourdomain.com     (Cloudflare Pages — your built HTML/JS)
       │
       └──►  todo-api.yourdomain.com  (Cloudflare Tunnel — no open ports at home)
                    │
                    ▼
              Go API + SQLite on YOUR computer (or NAS)
```

1. **Pages** serves the UI from static files (`public/` after you build the frontend).
2. **Tunnel** connects Cloudflare to your API on `127.0.0.1:8080`—only outbound traffic from your machine.
3. **SQLite** holds the data. You back it up like any file. Optionally, **`cogs sync`** pushes read copies to D1/R2.

The **`cogs` CLI** is the orchestrator: Wrangler for Cloudflare, migrations, env files, tunnel lifecycle, and redeploy shortcuts.

This repo is a **workspace**: several apps can live under `apps/`, listed in `cogs.workspace.json`.

---

## Try it in five minutes

**Prerequisites:** [Go 1.23+](https://go.dev/dl/), [Node.js](https://nodejs.org/) (for the todo UI), a [Cloudflare account](https://dash.cloudflare.com/) with a domain on it, and [Wrangler](https://developers.cloudflare.com/workers/wrangler/) (CoGS runs it via `npx`).

```bash
git clone https://github.com/irgipaulius/cogs.git
cd cogs
./cogs apps list
```

**First time** (creates Pages project, tunnel, DNS, D1/R2, builds UI, starts API):

```bash
./cogs launch todo -y
```

You’ll sign in with Wrangler and paste a Cloudflare API token with **Zone → DNS → Edit** so CoGS can create `todo` and `todo-api` hostnames. The token is saved in `apps/todo/.env` (never commit that file).

**Every day after** (UI build, Pages upload, migrations, restart API—skips reprovisioning):

```bash
./cogs redeploy todo
```

Open the URLs printed at the end. Local admin metrics (loopback only): `http://127.0.0.1:8080/admin`.

More detail: [Quickstart](docs/quickstart.md) · [Cloudflare setup](docs/cloudflare-setup.md) · [Install CLI without cloning](docs/install.md)

---

## Start your own app

Scaffold a new project, then run it locally before you touch Cloudflare:

```bash
./cogs new myapp --module github.com/YOU/myapp
cd myapp
cp .env.example .env    # set COGS_SESSION_SECRET
go run ./cmd/app serve -env .env
```

Add `cogs.launch.json`, register the app in `cogs.workspace.json`, and run `./cogs launch myapp -y` when you’re ready to go public.

---

## What you get

| Piece | Role |
|--------|------|
| **Pure Go SQLite** | No CGO; easy cross-compile to Linux ARM (NAS) and amd64 |
| **`cogs launch` / `redeploy`** | Full edge setup vs fast “ship my changes” |
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
