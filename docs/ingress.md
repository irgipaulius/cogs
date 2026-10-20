# Ingress: tunnel-first, direct optional

CoGS apps exposed through **Cloudflare Tunnel** should treat the tunnel as the **default public path**. Some routes (large uploads, streaming) may also accept **direct** HTTP to your host (port forward, reverse proxy, public bind).

The **`pkg/ingress`** middleware enforces that policy from **`cogs.ingress.json`** next to your `.env`.

The **todo example** does not ship this file — it stays tunnel-only out of the box.

## Behavior

| Request | Default routes | `allow_direct: true` routes |
|---------|----------------|-----------------------------|
| Through Cloudflare (Tunnel / orange-cloud) | Allowed | Allowed |
| Loopback (`127.0.0.1`, cloudflared → origin) | Allowed if `trust_loopback` (default **true**) | Allowed |
| Direct from internet (no CF headers) | **403** | Allowed if optional direct secret matches |

Cloudflare marks tunneled traffic with headers such as **`Cf-Connecting-Ip`**, **`Cf-Ray`**, **`Cf-Visitor`**. Direct clients hitting your public IP:port usually lack these.

You are responsible for **TLS, firewall, and port forwarding** on the direct path. CoGS only gates routes in application code.

## Config file

`cogs.ingress.json` (or path in **`COGS_INGRESS_CONFIG`**):

```json
{
  "default_policy": "tunnel_only",
  "trust_loopback": true,
  "direct_secret_env": "COGS_DIRECT_ACCESS_KEY",
  "routes": [
    { "pattern": "POST /api/bulk/*", "allow_direct": true },
    { "pattern": "GET /api/export/*", "allow_direct": true }
  ]
}
```

- **`pattern`**: `METHOD /path`, or `/path` for any method. Trailing `*` is prefix match.
- **`allow_direct`**: when **true**, clients without Cloudflare headers may call this route (subject to `direct_secret_env`).
- **`direct_secret_env`**: name of an env var; if set, direct callers must send header **`X-Cogs-Direct-Key`** with the same value.

Routes **not listed** still follow the default: tunnel (or loopback) only.

## Wiring in your app

Scaffolded apps (`cogs new`) load ingress automatically:

```go
ingressCfg, err := ingress.LoadOptional(ingress.ResolvePath(envFile))
// server.Deps{ Ingress: ingressCfg, ... }
```

If **`cogs.ingress.json` is missing**, middleware is **off** (all routes behave as today).

## Direct URL vs tunnel URL

Typical setup:

- **`https://api.example.com`** → Cloudflare Tunnel → `127.0.0.1:PORT` (everyday API)
- **`https://bulk.example.com:8443`** or **`https://your-ip:8443`** → Caddy/nginx → same Go process with **`COGS_ALLOW_PUBLIC_BIND=1`**

Point the frontend at the tunnel URL for normal calls; use the bulk base URL only for routes marked **`allow_direct`**.

## Limits

- **Tunnel** path: subject to Cloudflare HTTP body limits (~100 MiB on many plans).
- **Direct** path: subject to **`COGS_MAX_UPLOAD_BYTES`**, your proxy, and disk — not the tunnel cap.

See [cloudflare-setup.md](cloudflare-setup.md) and [docker.md](docker.md) for listen/bind notes.

## Schema

See [cogs.ingress.schema.json](cogs.ingress.schema.json).
