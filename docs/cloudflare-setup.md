# Cloudflare setup

## Tunnel (public hostname)

1. `cogs tunnel init api.example.com`
2. Install `cloudflared` on your server (`cogs deps --target linux/amd64` or package manager).
3. Route DNS to the tunnel; point ingress at `http://127.0.0.1:8080`.
4. Enable **Bot Fight Mode** and a **WAF rate limit** on the hostname (Free plan: 1 rule — see Cloudflare docs for current limits).

## Turnstile

1. Create a widget in the dashboard.
2. Set `COGS_TURNSTILE_SECRET` and `COGS_TURNSTILE_SITE_KEY`.
3. Clients call `POST /session` with the Turnstile token, then use the returned bearer token.

## Cache

Add a Cache Rule for cacheable `GET`s. Round bbox/query params client-side so keys repeat.

## Access (admin)

Protect `/admin/*` with an Access application and service token. Set `COGS_ACCESS_TEAM_DOMAIN` and `COGS_ACCESS_AUDIENCE`.

## D1 + R2

```bash
cogs cf d1 create cogs-edge
cogs cf r2 create cogs-data
```

Copy IDs into `.env`. Push data with `cogs sync`, not raw SQL dumps, for incremental updates.

## Private Worker → origin

See [Workers VPC private API](https://developers.cloudflare.com/workers-vpc/examples/private-api/) and `cogs tunnel private`.
