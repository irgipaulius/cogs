# FreeBSD (and jails)

Build on a modern host with `CGO_ENABLED=0`:

```bash
cogs build --target freebsd/amd64
```

## Recommended: one workspace supervisor (`cogs up`)

From the repo root after `cogs launch` on your apps:

```bash
cogs up              # background supervisor
# or: cogs up --foreground   # foreground (Docker-style)
```

- Writes **`.cogs/boot/cogs_up`** (rc.d script using **`daemon(8)`** as user **`cogs`**, not blocking boot).
- Writes **`.cogs/boot/cogs-up.sh`** for `@reboot` cron if you prefer.
- Sources **`.cogs/boot/cogs-up.env`** for supervisor-only vars (`COGS_DEPLOY_QUEUE_ID`, `COGS_CF_QUEUE_ID`).
- **`service cogs_up status|stop`** use the pidfile under **`.cogs/run/up.pid`** (written by `cogs up --foreground`).
- Restarts app binaries, tunnels, **admin supervisor**, and polls the deploy queue.

Install the generated rc script:

```bash
sudo install -m 555 .cogs/boot/cogs_up /usr/local/etc/rc.d/cogs_up
sysrc cogs_up_enable=YES
```

Stop everything: `cogs down`.

## Legacy single-app rc (optional)

Per-app scripts under `cogs service render rcd` still exist for one binary only:

```bash
cogs service render rcd
sudo install -m 555 cogs-app.rc /usr/local/etc/rc.d/cogs_app
```

Use that only when you are **not** using workspace `cogs up`.

Run `cloudflared` / wrangler tunnel in the same jail as the API so the tunnel reaches `COGS_LISTEN` (loopback in `cogs.app.toml`).

If `pkg install cloudflared` is unavailable, use `cogs deps --target freebsd/amd64`.

Python for goldilocks scraper: install `python3` or set **`COGS_PYTHON=/usr/local/bin/python3.11`**.
