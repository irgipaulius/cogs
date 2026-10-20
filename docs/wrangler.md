# Wrangler helpers

CoGS wraps common Wrangler commands as `cogs cf …`. Wrangler must be on `PATH` (or use `npx wrangler`).

| CoGS command | Wrangler equivalent |
|--------------|---------------------|
| `cogs cf whoami` | `wrangler whoami` |
| `cogs cf r2 create B` | `wrangler r2 bucket create B` |
| `cogs cf d1 create N` | `wrangler d1 create N` |
| `cogs cf pages deploy ./site` | `wrangler pages deploy ./site` |
| `cogs cf worker deploy` | `wrangler deploy` |
| `cogs cf kv namespace create N` | `wrangler kv namespace create N` |
| `cogs tunnel private` | `wrangler tunnel create` + VPC service docs |

Data sync to D1 is **`cogs sync`**, not `d1 execute --file`, so only changed rows upload after the initial load.
