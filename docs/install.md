# Installing the CoGS CLI

## Option A — curl install (released binaries)

Unix (linux, macOS, FreeBSD):

```bash
curl -fsSL https://raw.githubusercontent.com/irgipaulius/cogs/main/scripts/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
cogs version
```

Pin a version:

```bash
COGS_VERSION=v0.1.0 curl -fsSL .../install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/irgipaulius/cogs/main/scripts/install.ps1 | iex
```

Binaries are built by GoReleaser on git tags `v*`. Until the first release, use Option B.

## Option B — git clone + build (development)

```bash
git clone https://github.com/irgipaulius/cogs.git
cd cogs
./cogs apps list    # builds dist/cogs automatically
```

Or install globally:

```bash
go build -o "$(go env GOPATH)/bin/cogs" ./cmd/cogs
```

## Option C — go install

```bash
go install github.com/irgipaulius/cogs/cmd/cogs@latest
```

Requires a tagged module version on the module proxy.

## Host prerequisites (deploy + admin dashboard)

On the machine that runs **`cogs up`** and the **admin** app:

- [GitHub CLI](https://cli.github.com/) — `gh auth login && gh auth setup-git`
- [Wrangler](https://developers.cloudflare.com/workers/wrangler/) — `wrangler login` (or workspace DNS token; see [cloudflare-setup.md](cloudflare-setup.md))

One-shot script from the repo: **`scripts/bootstrap-host.sh`**.

Admin **Add app** and pull-mode Builds require **`gh`** on that host. See [deploy.md](deploy.md) and [admin-dashboard.md](admin-dashboard.md).
