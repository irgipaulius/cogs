# macOS

Development:

```bash
go run ./cmd/app serve -env .env
wrangler tunnel run my-tunnel
```

Production-style daemon:

```bash
cogs service render launchd
# edit paths, then launchctl load com.cogs.app.plist
```

Requires macOS 12+ for current Go releases (see go.dev/wiki/MinimumRequirements).
