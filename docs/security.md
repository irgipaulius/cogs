# Security / threat model

## Trust boundaries

- **Origin** binds `127.0.0.1` by default; only `cloudflared` should reach the app on the host.
- **Cloudflare edge** terminates TLS, applies WAF, bot management, and caching.
- **Turnstile + session token** reduces automated abuse on public routes without a DB hit per request.
- **Access JWT** gates `/admin/*`.
- **Private mode** uses Workers VPC + `COGS_PRIVATE_TOKEN`; no public DNS name required.

## What CoGS does not solve

- Compromised server host or stolen tunnel token.
- D1/R2 credentials in `.env` — treat as secrets, restrict file permissions.
- Multi-writer SQLite — one writer per database by design.

## API keys

Set `COGS_API_KEY` and use optional middleware for non-browser clients when Bot Fight Mode would block them.
