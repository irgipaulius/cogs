# Docker (optional)

CoGS defaults to **native deploy**: `./cogs launch`, `cogs deploy`, systemd on FreeBSD/Linux, SQLite on a host volume. Docker is an **optional runtime** for the API process—mainly Linux hosts and CI.

Cloudflare **Workers (UI), Tunnel, and DNS** are still provisioned with the **`cogs` CLI** on a machine where you can run Wrangler (laptop, CI, or admin box). The container typically runs **`cogs up --foreground`** (supervisor + apps) or **migrate + serve** only for a single API image.

## Workspace stack (todo + admin)

From the **repo root**, run todo and the public admin dashboard together:

```bash
cp .env.docker.example .env.docker   # admin login + **CORS** (`COGS_PUBLIC_PAGE_ORIGIN` = public UI origin → API)
docker compose up --build -d
```

If the todo UI on Cloudflare (Worker hostname) gets **CORS / Failed to fetch**, the API container is missing `COGS_PUBLIC_PAGE_ORIGIN` (must match `https://todo.<zone>`). Set it in `.env.docker` and `docker compose up -d todo-api --force-recreate`.

- **Admin UI**: http://127.0.0.1:18786 — default login `admin` / `CogsAdmin-Docker-2026!` (change in `.env.docker`).
- **Todo API**: http://127.0.0.1:18787/health
- Admin container mounts the repo at `/workspace`, runs **supervisor** + **admin** (socket under `/tmp` in Docker).

Stop the older single-app compose if port 18787 is busy: `docker compose -f apps/todo/docker-compose.yml down`.

## Todo app (API only)

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

Or **Compose** (from `apps/todo/`) — includes **MinIO** for attachments on the host stack:

```bash
cp .env.docker.example .env.docker   # optional overrides
docker compose -f docker-compose.yml up --build
```

- **Port**: `18787` API. MinIO stays on the Docker network (`minio:9000`); uncomment `ports` in compose to expose console/S3 on the host (defaults `19000`/`19001` if you set them).
- **SQLite**: `./data/todo.db` bind mount.
- **Blobs**: `COGS_BLOB_STORE=minio` → `http://minio:9000` bucket `cogs-todo-blobs` (not Cloudflare R2).
- **Large files**: compose sets `COGS_MAX_UPLOAD_BYTES=104857600` (100 MiB). For a **99 MiB** test, upload via **`http://127.0.0.1:18787`** (direct to Docker). The **tunnel hostname** may reject bodies near Cloudflare’s ~100 MiB limit even when the app allows more.

## Tunnel with Docker

Typical pattern:

1. Run **`cogs launch todo`** once to create tunnel + DNS + Worker UI.
2. Run the API in Docker on the same host (or another host that can reach the tunnel target).
3. Point tunnel ingress at **`http://127.0.0.1:18787`** (or the host port you publish).

Running `cloudflared` itself is often easier **on the host** (or a separate container on a Docker bridge). CoGS does not require Docker for tunnels.

## FreeBSD / no Docker

Use the same SQLite paths and **`cogs service render`** unit files under `cogstemplate/template/deploy/`—no container needed.

## Scaffold template

`cogs new` projects include a minimal **`FROM scratch`** Dockerfile for a pre-built static binary; see `apps/todo/Dockerfile` for a full multi-stage example.
