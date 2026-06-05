# scout — one-line installer (Windows).
#
#   irm https://raw.githubusercontent.com/aykutssert/scout/main/install.ps1 | iex
#
# Delegates to npm/scripts/install.js, which downloads the prebuilt scout binary
# for this OS/arch from GitHub Releases and copies the scout skill into every
# detected AI agent. Mirrors install.sh.

$ErrorActionPreference = "Stop"
$Repo = "aykutssert/scout" # repo holding the releases; renamed to scout later

if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
  Write-Error "scout: Node.js (>=18) required. Install: https://nodejs.org"
  exit 1
}

$nodeMajor = [int](node -p "process.versions.node.split('.')[0]")
if ($nodeMajor -lt 18) {
  Write-Error "scout: Node $nodeMajor too old, need >=18. Upgrade: https://nodejs.org"
  exit 1
}

# Running from a local clone: skip the network round-trip.
$here = $PSScriptRoot
if ($here -and (Test-Path (Join-Path $here "npm/scripts/install.js"))) {
  node (Join-Path $here "npm/scripts/install.js") @args
  exit $LASTEXITCODE
}

# Pipe path: shallow-clone the repo, then run the installer from it.
if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
  Write-Error "scout: git required for the install path."
  exit 1
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("scout-" + [System.Guid]::NewGuid().ToString("N"))
try {
  git clone --depth 1 "https://github.com/$Repo" "$tmp" 2>$null
  if ($LASTEXITCODE -ne 0) { Write-Error "scout: clone failed (https://github.com/$Repo)."; exit 1 }
  node (Join-Path $tmp "npm/scripts/install.js") @args
  exit $LASTEXITCODE
} finally {
  if (Test-Path $tmp) { Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue }
}
