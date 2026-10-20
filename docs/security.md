# Security / threat model

## Trust boundaries

- **Origin** binds `127.0.0.1` by default; only `cloudflared` should reach the app on the host.
- **Cloudflare edge** terminates TLS, applies WAF, bot management, and caching.
- **Turnstile + session token** reduces automated abuse on public routes without a DB hit per request.
- **Access JWT** (optional) can gate routes when configured.
- **Public admin** (`apps/admin`) uses username/password in `admin.db` with **no default credentials**; privileged ops (SQL, redeploy, cross-app metrics) run only via **`cogs supervisor`** on a unix socket, callable from the admin binary after session + RBAC checks.
- **Per-app `/admin`** (observe) stays **loopback Host only** — not the public dashboard.
- **Private mode** uses Workers VPC + `COGS_PRIVATE_TOKEN`; no public DNS name required.

## What CoGS does not solve

- Compromised server host or stolen tunnel token.
- D1/R2 credentials in `.env` — treat as secrets, restrict file permissions.
- Multi-writer SQLite — one writer per database by design.

## API keys

Set `COGS_API_KEY` and use optional middleware for non-browser clients when Bot Fight Mode would block them.
