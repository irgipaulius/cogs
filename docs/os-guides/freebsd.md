# FreeBSD (and jails)

Build on a modern host with `CGO_ENABLED=0`:

```bash
cogs build --target freebsd/amd64
```

Install binary to `/usr/local/bin/cogs-app`, render `cogs service render rcd`, enable in `/etc/rc.conf`:

```
cogs_enable="YES"
```

Run `cloudflared` in the same jail as the app so `127.0.0.1:8080` is reachable.

If `pkg install cloudflared` is unavailable, use `cogs deps --target freebsd/amd64`.
