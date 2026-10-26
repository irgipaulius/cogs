# Quickstart

CoGS (**C**loudflare + **G**o + **S**QLite) is a forkable boilerplate for self-hosted backends that pair with Cloudflare’s edge.

## 5-minute path

1. **Install the CLI**: clone + `./cogs` from repo root, or [install.md](install.md) (`curl | sh` after a release)
2. **Scaffold an app**: `cogs new myapp --module github.com/YOU/myapp`
3. **Configure**: `cd myapp && cp .env.example .env` — set `COGS_SESSION_SECRET`.
4. **Run**: `go run ./cmd/app serve -env .env` → `curl http://127.0.0.1:8080/health`
5. **Try the todo demo**: `./cogs launch todo -y` from repo root (Wrangler login + DNS token prompt)
6. **Ship changes**: `./cogs redeploy todo` (Worker UI + backend release)
7. **Host supervisor (prod loop)**: `cogs up` — see [deploy.md](deploy.md) (also runs optional per-app DB backups when enabled — [backups.md](backups.md))
8. **Admin Add app / private repos**: `gh auth login` on the server — [admin-dashboard.md](admin-dashboard.md)
9. **Build static binary**: `cogs build --target linux/amd64`
10. **Expose** (pick one):
   - Public: `cogs tunnel init api.example.com` + [cloudflare-setup.md](cloudflare-setup.md)
   - Worker-only: `cogs tunnel private` + [worker-client example](../examples/worker-client)

## Sync to the edge

Edit `sync.json`, set `COGS_D1_*` and R2 vars, then:

```bash
cogs sync schema   # once (via app migrate + docs)
cogs sync          # fast path: outbox only
cogs sync --checksum   # drift check
```

SQLite remains the source of truth; D1 and R2 are read-optimized projections.
