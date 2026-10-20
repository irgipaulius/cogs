# FreeBSD (and jails)

Build on a modern host with `CGO_ENABLED=0`:

```bash
cogs build --target freebsd/amd64
```

Install binary to `/usr/local/bin/cogs-app`, then:

```bash
cogs service render rcd
sudo install -m 555 cogs-app.rc /usr/local/etc/rc.d/cogs_app
```

Enable in `/etc/rc.conf` (script runs the app in the background via `daemon(8)`):

```
cogs_app_enable="YES"
```

Run `cloudflared` in the same jail as the app so the tunnel can reach your `COGS_LISTEN` address (see app `.env`).

If `pkg install cloudflared` is unavailable, use `cogs deps --target freebsd/amd64`.
