#!/usr/bin/env bash
# scout — one-line installer.
#
#   curl -fsSL https://raw.githubusercontent.com/aykutssert/scout/main/install.sh | bash
#
# Delegates to npm/scripts/install.js (the unified Node installer): it downloads
# the prebuilt scout binary for this OS/arch from GitHub Releases and copies the
# scout skill into every detected AI agent (Claude Code, Codex, ...). All flags
# are forwarded to install.js.
#
# Why Node, not pure bash? One installer avoids bash/PowerShell drift and works
# the same everywhere. Run `scout install` later to repair/re-run.

set -euo pipefail

REPO="aykutssert/scout" # repo holding the releases; renamed to scout later

if ! command -v node >/dev/null 2>&1; then
  echo "scout: Node.js (>=18) required." >&2
  echo "  macOS:  brew install node" >&2
  echo "  Linux:  https://nodejs.org or nvm (https://github.com/nvm-sh/nvm)" >&2
  exit 1
fi

NODE_MAJOR="$(node -p 'process.versions.node.split(".")[0]')"
if [ "$NODE_MAJOR" -lt 18 ]; then
  echo "scout: Node $NODE_MAJOR too old, need >=18. Upgrade: https://nodejs.org" >&2
  exit 1
fi

# Running from a local clone: skip the network round-trip.
here="$(cd "$(dirname "${BASH_SOURCE[0]:-}")" 2>/dev/null && pwd)" || here=""
if [ -n "$here" ] && [ -f "$here/npm/scripts/install.js" ]; then
  exec node "$here/npm/scripts/install.js" "$@"
fi

# Curl-pipe path: shallow-clone the repo, then run the installer from it.
if ! command -v git >/dev/null 2>&1; then
  echo "scout: git required for the curl install path." >&2
  exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
git clone --depth 1 "https://github.com/$REPO" "$tmp/scout" >/dev/null 2>&1 || {
  echo "scout: clone failed (https://github.com/$REPO)." >&2
  exit 1
}
exec node "$tmp/scout/npm/scripts/install.js" "$@"
