# Install cogs CLI from GitHub releases (Windows amd64/arm64).
# Usage:
#   irm https://raw.githubusercontent.com/irgipaulius/cogs/main/scripts/install.ps1 | iex
param(
  [string]$Version = $env:COGS_VERSION,
  [string]$InstallDir = $env:COGS_INSTALL_DIR,
  [string]$Repo = $(if ($env:COGS_GITHUB_REPO) { $env:COGS_GITHUB_REPO } else { "irgipaulius/cogs" })
)

$ErrorActionPreference = "Stop"

if (-not $InstallDir) {
  $InstallDir = Join-Path $env:LOCALAPPDATA "cogs\bin"
}

$arch = $env:PROCESSOR_ARCHITECTURE
switch ($arch) {
  "AMD64" { $goarch = "amd64" }
  "ARM64" { $goarch = "arm64" }
  default { throw "unsupported arch: $arch" }
}
$goos = "windows"

if (-not $Version -or $Version -eq "latest") {
  $rel = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
  $Version = $rel.tag_name
}

$tag = $Version
if ($tag -notmatch "^v") { $tag = "v$tag" }

$ver = $tag.TrimStart("v")
$asset = "cogs_${ver}_windows_${goarch}.zip"
$url = "https://github.com/$Repo/releases/download/$tag/$asset"

$tmp = Join-Path $env:TEMP ("cogs-install-" + [guid]::NewGuid().ToString("n"))
New-Item -ItemType Directory -Path $tmp -Force | Out-Null
$zip = Join-Path $tmp $asset

Write-Host "Downloading $url …"
Invoke-WebRequest -Uri $url -OutFile $zip
Expand-Archive -Path $zip -DestinationPath $tmp -Force

$bin = Join-Path $tmp "cogs.exe"
if (-not (Test-Path $bin)) { throw "archive did not contain cogs.exe" }

New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Copy-Item -Path $bin -Destination (Join-Path $InstallDir "cogs.exe") -Force

Write-Host "Installed cogs to $InstallDir\cogs.exe ($tag, windows/$goarch)"
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
  Write-Host "Add to user PATH: $InstallDir"
}
