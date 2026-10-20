# Docker (optional)

CoGS defaults to **native deploy**: `./cogs launch`, `cogs deploy`, systemd on FreeBSD/Linux, SQLite on a host volume. Docker is an **optional runtime** for the API process—mainly Linux hosts and CI.

Cloudflare **Pages, Tunnel, and DNS** are still provisioned with the **`cogs` CLI** on a machine where you can run Wrangler (laptop, CI, or admin box). The container runs **migrate + serve** only.

## Todo app

From the **repo root**:

```bash
docker build -f apps/todo/Dockerfile -t cogs-todo .
docker run --rm -p 18787:18787 \
  -e COGS_SESSION_SECRET="$(openssl rand -hex 16)" \
  -e COGS_PUBLIC_PAGE_ORIGIN=https://todo.example.com \
  -e COGS_PUBLIC_API_URL=https://todo-api.example.com \
  -v cogs-todo-data:/data \
  cogs-todo
curl -sS http://127.0.0.1:18787/health
```

Or **Compose** (from `apps/todo/`):

```bash
cp .env.docker.example .env.docker   # edit secrets
docker compose -f docker-compose.yml up --build
```

- **Port**: `18787` inside the container (`COGS_LISTEN=0.0.0.0:18787`). Map with `COGS_HOST_PORT` in compose.
- **Data**: SQLite at `/data/todo.db` (named volume `todo-data`).
- **Config**: environment variables and/or `.env.docker` mounted as `/config/.env` if you add that bind in compose.

## Tunnel with Docker

Typical pattern:

1. Run **`cogs launch todo`** once to create tunnel + DNS + Pages.
2. Run the API in Docker on the same host (or another host that can reach the tunnel target).
3. Point tunnel ingress at **`http://127.0.0.1:18787`** (or the host port you publish).

Running `cloudflared` itself is often easier **on the host** (or a separate container on a Docker bridge). CoGS does not require Docker for tunnels.

## FreeBSD / no Docker

Use the same SQLite paths and **`cogs service render`** unit files under `cogstemplate/template/deploy/`—no container needed.

## Scaffold template

`cogs new` projects include a minimal **`FROM scratch`** Dockerfile for a pre-built static binary; see `apps/todo/Dockerfile` for a full multi-stage example.
