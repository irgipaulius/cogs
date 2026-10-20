# CoGS: Cloudflare + Go + SQLite — build plan

## 1. Pitch

CoGS is an open-source boilerplate and small CLI for self-hosting backends that serve Cloudflare apps. **C** = Cloudflare (edge: Tunnel, WAF, Cache, Turnstile, Access, R2, Pages/Workers as clients). **G** = Go (one static, CGO-free binary per role). **S** = SQLite (one file per project, via `modernc.org/sqlite`). The problem: Workers and D1 have request, CPU, and size limits, and your data lives on someone else's platform. CoGS runs the backend and data on any machine you control (bare metal, VPS, Linux, macOS, old FreeBSD jails), reaches the internet only through `cloudflared`, and still uses Cloudflare's edge for caching and abuse protection. No containers required. Docker is an optional extra, never a dependency.

Targets: `linux`, `freebsd`, `darwin` × `amd64`, `arm64`, all built with `CGO_ENABLED=0`.

## 2. Architecture

```
browser / Pages / Worker ──HTTPS──> Cloudflare edge (WAF, cache, bot, Access)
                                         │ tunnel
                                    cloudflared ──> 127.0.0.1:8080  app (Go)
                                                                     ├─ data.db (SQLite, WAL)
                                                                     ├─ R2 (S3 API)
                                                                     └─ jobs (scheduler)
                                    litestream ──> R2 (backup)
```

**One project = one backend + one db.** There's no shared db server. Projects are isolated by directory and OS user.

**Database (`internal/db`)**
- Driver: `modernc.org/sqlite` (pure Go). Open via `database/sql` with this DSN:
  `_txlock=immediate`, `journal_mode(WAL)`, `busy_timeout(5000)`, `synchronous(NORMAL)`, `foreign_keys(ON)`, `cache_size(-65536)`, `mmap_size(1<<30)`, `temp_store(MEMORY)`.
- Use two pools: a writer (`MaxOpenConns=1`) and a reader (`MaxOpenConns=N`). This avoids `SQLITE_BUSY` storms.
- `page_size` is set before the first table is created (default 4096; 16384 is an option). The docs tell users to match the filesystem block/record size (e.g. ZFS `recordsize`), and to keep the db on SSD.
- Migrations: **goose** with `embed.FS`, run on start (`goose.Up`). It takes plain `.sql` files, works with any `database/sql` driver, and has a library API. golang-migrate's sqlite drivers are cgo or a separate module.
- Queries: **sqlc** generates typed Go from `.sql`, reading the migrations as schema.

**Geo module (optional, `internal/geo`)**
- An R-tree virtual table (`rtree(id, min_x, max_x, min_y, max_y)`) kept in sync in the same transaction as row writes. It provides a bbox query helper and antimeridian splitting.
- R-tree stores 32-bit floats. Re-filter exact coordinates in SQL/Go when precision matters.
- **Unverified:** that modernc builds SQLite with `SQLITE_ENABLE_RTREE`. M1 tests this. If it's missing, the module falls back to a `(lon, lat)` index with a range scan.
- **Unverified:** that sqlc parses `CREATE VIRTUAL TABLE ... USING rtree`. If it doesn't, geo queries stay handwritten.

**HTTP (`internal/server`)**
- stdlib `net/http` with Go 1.22+ method/path patterns. No third-party router.
- Binds to `127.0.0.1` by default. Refuses `0.0.0.0` unless `COGS_ALLOW_PUBLIC_BIND=1` is set.
- `ReadHeaderTimeout`/`WriteTimeout`, graceful shutdown on SIGTERM, JSON helpers, and `Cache-Control` helpers for cacheable GETs.
- Built-ins:
  - `GET /health`: runs `SELECT 1` and reports the migration version.
  - `GET /metrics`: Prometheus text format (request counts/latency, db pool stats, job last-run), Access-protected.
  - `POST /session`: Turnstile exchange.

**Exposure**
- `cloudflared` tunnel to `api-<project>.<domain>`. Clients call it **directly**, with no Worker gating it, so Workers request limits don't apply. The Cloudflare proxy still has its own limits, e.g. a request-body cap of 100 MB on Free (from Cloudflare docs, re-check).
- **Unverified:** that Cloudflare publishes no official FreeBSD cloudflared binary. A FreeBSD port (`net/cloudflared`) exists, also unverified. CoGS builds cloudflared from source for any target it lacks.

**Protection layers (outer → inner)**
1. WAF rate-limiting rule on the hostname. **Unverified:** Free-plan specifics (as of 2024: 1 rule, per-IP, 10 s period).
2. Bot Fight Mode. It can challenge legitimate non-browser clients. **Unverified:** whether a per-path skip is possible on Free.
3. Turnstile → app token. The client solves Turnstile, then `POST /session`. The server verifies with `challenges.cloudflare.com/turnstile/v0/siteverify` and returns an HMAC-signed token (default 15 min, bound to IP). The middleware checks the HMAC with no db hit.
4. Cache Rules for cacheable GETs. The docs recommend client-side bbox/grid rounding so cache keys repeat.
5. In-app limiter: `golang.org/x/time/rate` in an LRU keyed on `CF-Connecting-IP`, which is trusted only because the listener is localhost-only behind cloudflared. A second limiter is keyed on the session token. Over the limit returns 429 + `Retry-After`.
6. `/admin/*`: Cloudflare Access with a service-token policy. The middleware validates the `Cf-Access-Jwt-Assertion` JWT against `https://<team>.cloudflareaccess.com/cdn-cgi/access/certs`.

API-key auth is an optional middleware for non-browser clients that layers 2–3 would block.

**R2 (`internal/r2`)**
- `aws-sdk-go-v2/service/s3` with `BaseEndpoint=https://<account>.r2.cloudflarestorage.com` and region `auto`.
- Helpers: `PutCompressed` (zstd via `klauspost/compress`), `Get`, presigned URLs.
- **Unverified:** browser support for `Content-Encoding: zstd` in Safari. The default is to store `.zst` and leave decompression to the client.

**Scheduled jobs (`internal/jobs`)**
- Two modes:
  - (a) A built-in scheduler inside the app binary (`robfig/cron/v3`, or a small ticker-based one), recording last-run/last-error in a `_cogs_jobs` table.
  - (b) `app job <name>` as a one-shot subcommand for OS cron/systemd timers/launchd.
- Overlapping runs are prevented with a lock row in SQLite (`INSERT ... ON CONFLICT DO NOTHING` with expiry).

**Backups**
- Litestream replicating to R2. A config template ships with the repo.
  - **Unverified:** that current Litestream builds for freebsd with `CGO_ENABLED=0` (older versions used cgo SQLite).
  - Fallback: a built-in `app backup` that runs `VACUUM INTO` → zstd → R2 on a schedule.
- Filesystem snapshots, generic guidance: ZFS snapshots + `zfs send` to another pool/host, LVM/btrfs snapshots, or plain `sqlite3 .backup`. A snapshot of a WAL db is crash-consistent.
- `cogs restore-test` restores to a temp file and runs `PRAGMA integrity_check`.

**Service management (`deploy/`)**
- Templates rendered by `cogs new`:
  - systemd unit (+ timer for cron mode).
  - FreeBSD `rc.d` script using `daemon(8) -r -R 5 -P pidfile -u user`. **Unverified:** `daemon -o logfile` availability on very old releases; there, use `-S` (syslog).
  - launchd plist.
  - newsyslog/logrotate configs.
- An optional `Dockerfile` (scratch image) for those who want it.

**Config**
- Env vars only (`COGS_*` + app-specific), loaded from an env file path passed with `-env`. A typed struct with validation runs at start and fails fast with a list of missing vars. No YAML.

**Go/OS support facts**
- Verified (go.dev/wiki/FreeBSD): FreeBSD 11 support ended at Go 1.19.x; 12-STABLE and 13-STABLE are listed as supported.
- **Unverified:** whether the current Go release still supports FreeBSD 12. Check the release notes.
- Verified (go.dev/wiki/MinimumRequirements): Linux kernel ≥ 3.2 (Go 1.24+); macOS 12+ for Go 1.25/1.26, 13+ from Go 1.27.
- Jails use the host kernel, so the host version is what matters.
- Recommend cross-compiling over building on old hosts.

## 3. Repo layout

```
cogs/
  cmd/cogs/                 # the CLI
  template/                 # what `cogs new` copies (Go text/template)
    cmd/app/main.go         # serve | job <name> | migrate | backup | version
    internal/config/        # env struct + validation
    internal/db/            # open, pragmas, reader/writer pools, goose runner
    internal/db/migrations/ # 0001_init.sql
    internal/db/query/      # sqlc .sql -> generated code
    internal/server/        # mux, middleware (limiter, session, access, logging)
    internal/jobs/          # scheduler + registry
    internal/r2/            # S3 client + zstd helpers
    internal/geo/           # optional R-tree module (--with geo)
    deploy/systemd/  deploy/rc.d/  deploy/launchd/  deploy/logrotate/
    deploy/cloudflared.yml.tmpl  deploy/litestream.yml.tmpl
    Makefile  sqlc.yaml  .env.example  Dockerfile (optional)
  pkg/                      # importable libs used by template (limiter, session, accessjwt, sqliteopen)
  examples/geo-points/      # full example app: scraper job + bbox API + R2 export
  tools/build-deps/         # scripts to build cloudflared + litestream for any target
  docs/                     # quickstart, cloudflare-setup, security, backups, os-guides/
  .github/workflows/        # ci.yml, release.yml
```

Decision: shared logic lives in `pkg/` as a versioned Go module, so generated apps get fixes via `go get -u`. The template stays thin: wiring plus app code.

**CLI**
- `cogs new <project> [--module github.com/x/y] [--with geo,jobs]`: render the template, `go mod tidy`, `git init`.
- `cogs build [--target freebsd/amd64,...]`: wraps `CGO_ENABLED=0 GOOS GOARCH go build -trimpath -ldflags "-s -w -X version"` and puts the output in `dist/<os>_<arch>/`.
- `cogs deps --target <os/arch>`: build or download cloudflared and litestream for the target.
- `cogs deploy <ssh-host> [--target ...] [--service systemd|rcd|launchd]`: scp the binary, atomically swap it (keeping `.prev`), restart the service, poll `/health`, and roll back on failure.
- `cogs tunnel init <hostname>`: wraps `cloudflared tunnel create` + `route dns` and renders the config.
- `cogs service render <kind>`: print or install the service file.
- `cogs restore-test`, `cogs doctor` (checks pragmas, bind address, env, and cloudflared reachability).

**Makefile targets (in the generated app)**: `build`, `build-all`, `test`, `sqlc`, `migrate-new name=...`, `run`, `deploy HOST=... TARGET=...`, `rollback HOST=...`, `deps TARGET=...`. They call `cogs` when it's installed and fall back to raw `go build`.

## 4. Milestones

**M0: Feasibility spikes**
- Deliverables: a throwaway binary that opens modernc SQLite with pragmas, creates an R-tree, and runs a bbox query; plus build attempts of cloudflared and litestream with `CGO_ENABLED=0` for all 6 targets.
- Accepted when: the binary runs on linux/amd64, linux/arm64, darwin/arm64, and a FreeBSD 13 VM; there's a written yes/no for R-tree, sqlc+rtree, and the cloudflared/litestream FreeBSD builds; and fallbacks are chosen where the answer is no.

**M1: Core libs (`pkg/`)**
- Deliverables: `sqliteopen` (pools + pragmas), `config`, `limiter`, `session` (Turnstile+HMAC), `accessjwt`, `r2`.
- Accepted when: unit tests pass and the limiter is tested with a fake clock.
  - The session tests cover a token that is expired, tampered with, or presented from the wrong IP.
  - The accessjwt tests use a local JWKS fixture.
  - The `r2` tests run against MinIO in CI (Docker in CI only).

**M2: Template app**
- Deliverables: `serve`, `migrate`, `/health`, `/metrics`, goose+embed, sqlc, middleware chain.
- Accepted when: `go run ./cmd/app serve` works from a fresh copy; the app refuses a public bind; the migration version shows in `/health`; and a concurrent read/write test shows no `SQLITE_BUSY`.

**M3: Jobs + backups**
- Deliverables: the scheduler and the `job` subcommand, the lock table, `backup` (VACUUM INTO→zstd→R2), the litestream template, and `restore-test`.
- Accepted when: overlapping job runs are prevented in a test, and the backup→restore round trip passes `integrity_check`.

**M4: CLI**
- Deliverables: `new`, `build`, `deps`, `service render`, `tunnel init`, `deploy` with rollback.
- Accepted when: `cogs new demo && cd demo && cogs build --target freebsd/amd64` produces a static binary (`file` shows "statically linked"), and `deploy` to a test VM rolls back automatically when `/health` fails.

**M5: Service templates + OS guides**
- Deliverables: systemd, rc.d, launchd, and log rotation templates, plus docs for Debian/Ubuntu, FreeBSD (incl. jails), and macOS.
- Accepted when: each template is tested on a real VM (start, crash→restart, stop, reboot).

**M6: Example app (`examples/geo-points`)**
- Deliverables: a scrape job → upsert → R-tree; a bbox API; a zstd export to R2; and a static Pages client that uses Turnstile.
- Accepted when: it runs end to end via a tunnel on a test domain, and cache hits are visible in the `cf-cache-status` header.

**M7: Cloudflare setup docs + security review**
- Deliverables: step-by-step docs for the tunnel, WAF rule, Bot Fight Mode, Cache Rules, Access app + service token, and Turnstile widget, plus a threat-model page.
- Accepted when: someone new to the project goes from zero to a protected endpoint by following the docs alone.

**M8: CI + release**
- Deliverables:
  - GitHub Actions `ci.yml`: `go vet`, `staticcheck`, tests, and `sqlc diff`, with a cross-compile matrix of {linux,freebsd,darwin}×{amd64,arm64} using `CGO_ENABLED=0`, and a FreeBSD test job (`vmactions/freebsd-vm`, **unverified** that it's still maintained).
  - `release.yml`: on tag, goreleaser builds `cogs` for 6 targets, plus checksums and SBOM, and attaches them to the GitHub Release.
- Accepted when: a tag produces 6 downloadable `cogs` binaries, and `cogs new` from a release binary builds the example.

**M9: v0.1.0 public**
- Deliverables: the README (pitch, 5-minute quickstart, limits), LICENSE (MIT or Apache-2.0), CONTRIBUTING, and a changelog.
- Accepted when: the GitHub repo is public with working boilerplate, the example app, docs, and green CI.

## 5. Decisions and tradeoffs

- **modernc vs mattn/go-sqlite3:** modernc has no cgo, so it cross-compiles trivially. It's somewhat slower (typically cited as up to ~2× on writes, unverified for current versions). That's acceptable for the target scale.
- **SQLite per project vs Postgres:** zero ops, file-level backups, and a single writer. Not for heavy multi-writer workloads; the docs say so.
- **stdlib router:** no dependency, and good enough since Go 1.22.
- **goose + sqlc:** plain SQL everywhere, no ORM. The cost is that dynamic queries have to be hand-written.
- **Env-only config:** simple, and fits every service manager.
- **Direct tunnel access, no Worker gate:** avoids Worker limits. The cost is that edge-side protection relies on Cloudflare rules plus the in-app layer instead of custom Worker logic. Workers can still call the backend as just another client.
- **pkg/ module + thin template:** apps get fixes without regenerating. The cost is maintaining API stability in `pkg/`.

## 6. Open questions

- License: MIT or Apache-2.0?
- Ship `cloudflared`/`litestream` builds in releases, or only scripts? (Redistribution licenses look permissive: Apache-2.0 for both, unverified.)
- Should the built-in scheduler be on by default, or should OS cron be the default?
- Should D1 sync/export be included (e.g., pushing read-only snapshots to D1 for Worker-side reads)?
- Should Windows be a target? It's currently out of scope.
- `/metrics` format: Prometheus text only, or also JSON?

## 7. Name

Checked 2026-10-20 via the GitHub API and pkg.go.dev:
- **`cogs`**: crowded. The GitHub org/user `github.com/cogs` is taken, and there are ~16k repos with "cogs" in the name. pkg.go.dev has several existing `cogs` packages (e.g. `github.com/mkatychev/cogs`, `github.com/kinematic-ci/cogs`).
- **`cogstack`**: taken by an active medical-NLP org (CogStack, MedCAT ~500★). Avoid.
- **`cogs-stack`**: the GitHub org is free (404), and no Go module exists. But ~12 repo-name hits are mostly CogStack, so it's confusing.
- **Alternatives:**
  - **`edgeroot`**: org free, 0 repo hits.
  - **`sqedge`**: org free, 0 repo hits.
  - `gocogs`: org taken (200), 0 repo hits.
  - `tunnelbase`: org taken (200), 0 repo hits.

Recommendation: keep "CoGS" as the stack name in docs if wanted, but publish under a free org, e.g. `github.com/cogs-stack/cogs`, or rename to `edgeroot` / `sqedge`. Neither "taken" nor "free" checks trademarks.
