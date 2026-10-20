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
