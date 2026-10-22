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

## Split-app UI (Workers static assets)

CoGS deploys split-app frontends as **assets-only Workers** (no `main` script) via `cogs launch` / `cogs redeploy` in **push** mode. Custom hostname + DNS are managed like tunnel records (see launch output).

## Pull mode (Workers Builds)

For **`deploy_mode = pull`** apps (e.g. goldilocks):

1. Install the **Cloudflare GitHub App** once: dashboard → Workers & Pages → Builds → Connect (the API cannot install it).
2. On the CoGS host: `gh auth login && gh auth setup-git` and `wrangler login` (or `scripts/bootstrap-host.sh`).
3. `cogs launch <app> -y` creates repo connection + build trigger.
4. **`cogs launch <pull-app> -y`** tries to create queue **`cogs-deploys`**, an **HTTP pull consumer**, and event subscriptions (best-effort; API errors are printed).
5. Queue id lands in **`.cogs/boot/cogs-up.env`**. Restart **`cogs up`** so it loads `COGS_DEPLOY_QUEUE_ID`.

Full steps and Pages → Worker cutover: [deploy.md](deploy.md).

## Private Worker → origin

See [Workers VPC private API](https://developers.cloudflare.com/workers-vpc/examples/private-api/) and `cogs tunnel private`.

## Direct (bulk) routes alongside tunnel

Tunnel is the default public path. For large uploads that exceed Cloudflare HTTP limits, opt specific routes into **direct** access (port forward + TLS on your side) via **`cogs.ingress.json`** — see [ingress.md](ingress.md). The todo example does not use this; scaffolded `cogs new` apps include a sample file.
