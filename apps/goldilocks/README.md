# Goldilocks (CoGS app)

World climate map backed by SQLite on your server: leveled cells, baked JSON tiles, MinIO cache, and a background TerraClimate scraper for native ~4.6 km data.

## Local (host)

```bash
cd apps/goldilocks
GOWORK=off go run ./cmd/app migrate -env .env.example
GOWORK=off go run ./cmd/app seed -env .env.example \
  -source /path/to/WIP_map/data/goldilocks.sqlite \
  -areas public/data/areas.json
GOWORK=off COGS_LISTEN=127.0.0.1:18788 COGS_DB_PATH=./data/goldilocks.db \
  go run ./cmd/app serve -env .env.example
```

Open http://127.0.0.1:18788/

## Docker (workspace stack)

From the CoGS repo root:

```bash
docker compose up --build -d goldilocks-api
```

API and UI: http://127.0.0.1:18788/health  
Seed mount: `WIP_map/data/goldilocks.sqlite` → `/seed/goldilocks.sqlite` (first boot imports automatically).

## API

| Route | Purpose |
|-------|---------|
| `GET /world/overview.json` | 2° cells (same shape as static Goldilocks) |
| `GET /world/t/{south}_{west}.json` | 0.5° cells in a 10° tile |
| `GET /api/tiles/{z}/{x}/{y}` | Map tile with level fallback |
| `GET /api/cells?country=&s=&w=&n=&e=` | Filtered cell rows |

## Scraper

`scripts/scrape_chunk.py` runs in the container every ~45s. Mount TerraClimate NetCDFs at `/tmp/tc` to fill level 3 (1/24°).
