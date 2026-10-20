# Observability (CoGS)

CoGS ships an opinionated **`pkg/observe`** layer: structured **`log/slog`** access logs, optional log file, and a **localhost-only** admin UI.

## Environment

| Variable | Values | Default |
|----------|--------|---------|
| `COGS_OBS` | `off`, `file`, `dashboard` | `file` |
| `COGS_OBS_LOG` | path to append logs | (empty = stderr only) |

- **`file`** — access logs + **`http://127.0.0.1:<port>/admin`** (local Host only).
- **`dashboard`** — same as `file` (alias for future extras).
- **`off`** — no logging, no admin UI.

## Admin dashboard

Open **`http://127.0.0.1:8080/admin`** on the machine running the API (not via `todo-api.yourdomain`).

Shows:

- Live log stream (SSE)
- CPU / RAM (via [gopsutil](https://github.com/shirou/gopsutil))
- HTTP counters
- SQLite file size + page stats + SQL query/exec counts when the app uses `observe.InstrumentDB`

## Libraries

- **Logging:** Go stdlib **`log/slog`**
- **Host metrics:** **`github.com/shirou/gopsutil/v3`**
- **Client ↔ server link tests:** **`pkg/linkperf`** — see [linkperf.md](./linkperf.md)
- **Prometheus:** optional; `GET /metrics` on the template app can be extended — observe focuses on local owner UX

## App wiring

```go
obs, err := observe.Setup(cfg)
defer obs.Close()
slog.SetDefault(obs.Logger)

server.NewMux(server.Deps{
    Config:  cfg,
    DB:      pools.Reader,
    Observe: obs,
    // ...
})
```

Wrap the SQLite writer for SQL counters:

```go
dbWriter := observe.InstrumentDB(pools.Writer, obs.SQL)
```
