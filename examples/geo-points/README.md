# geo-points example

Scrape-style job → SQLite upsert → optional geo bbox API → zstd export to R2 → `cogs sync` to D1.

## Run locally

```bash
cp .env.example .env
# set COGS_SESSION_SECRET
go run ./cmd/app serve -env .env
go run ./cmd/app job scrape -env .env
cogs sync -env .env
```

## Cloudflare

- Public API: `cogs tunnel init api.example.com` + Turnstile + Cache Rules (see [docs/cloudflare-setup.md](../../docs/cloudflare-setup.md)).
- Worker reads D1/R2; live scrape stays on origin via `cogs tunnel private`.
