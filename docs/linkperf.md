# Link performance (`pkg/linkperf`)

CoGS registers standard HTTP probes so **browsers** can measure latency and throughput to your API (including over Cloudflare Tunnel).

## Endpoints (on by default)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/link/ping` | JSON `{"t": ms}` — client measures RTT |
| GET | `/link/bench?bytes=N` | Download `N` bytes (default 128 KiB, max `COGS_LINKPERF_MAX_BYTES`) |
| POST | `/link/bench?bytes=N` | Upload body up to `N` bytes |

Disable with `COGS_LINKPERF_OFF=1`.

## Server

Registered automatically from `server.NewMux` when `config.App.LinkPerf` is true.

## Browser client

Reference implementation: `pkg/linkperf/client.js` (copied into app frontends as needed).

```javascript
import { measureLink } from "./lib/linkperf.js";
const { pingMs, downloadMbps, uploadMbps } = await measureLink("https://todo-api.example.com");
```

Measures median of 3 pings, then small download/upload payloads to estimate Mbps.

## Notes

- This is **application-layer** RTT/throughput (HTTP), not ICMP ping.
- Large `bytes` values consume bandwidth; cap with `COGS_LINKPERF_MAX_BYTES` (default 512 KiB).
