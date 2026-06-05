# scout

Deterministic code security & quality scanner for AI coding agents. Scans a
repo for **security, performance, correctness, and architecture** issues across
JS/TS frameworks (React, Next, Vite, TanStack, React Native, Node, Express,
NestJS, Bun, Svelte) and produces a **0–100 health score**. No LLM inside —
findings are deterministic; the agent reads them and fixes.

## Install

Needs Node ≥18. Installs the prebuilt binary for your OS/arch and drops the
`scout` skill into every AI agent it detects (Claude Code, Codex, …).

**npm (recommended):**
```bash
npm install -g @aykutss/scout
```

**npx (no global install):**
```bash
npx @aykutss/scout install
```

**One-line script (optional):**
```bash
# macOS / Linux / WSL
curl -fsSL https://raw.githubusercontent.com/aykutssert/scout/main/install.sh | bash
# Windows (PowerShell)
irm https://raw.githubusercontent.com/aykutssert/scout/main/install.ps1 | iex
```

**Manual (most transparent):** download `scout-<os>-<arch>` from
[Releases](https://github.com/aykutssert/scout/releases), verify its checksum,
put it on your `PATH`. Re-run `scout install` any time to repair or re-copy
skills.

## Use

In an AI agent — type **`/scout`** (or "scan this repo", "check health"). The
agent runs `scout doctor` to check tools, installs anything missing with your
permission, then runs the scan and reports findings with fixes.

Directly in a terminal:
```bash
scout scan              # scan current directory
scout scan --diff       # only files changed in git
scout doctor            # check system deps and toolchains
scout context <target>  # cross-file context for a symbol or file
```

`scan` prints a human-readable table on a terminal and JSON when piped (so an
agent can read it). A missing scanner is reported as an error, never a silent
pass: a clean report means the tools actually ran.

## How it works

```
        core (scan orchestrator)  ->  0–100 health score
                  |
  semgrep   osv   tree-sitter   oxlint   git-log
  (rules)  (CVE)    (graph)     (lint)  (history)
```

Proven linters are wrapped (semgrep, oxlint, eslint+ts/svelte, osv); what they
miss is caught by custom rules (semgrep YAML + tree-sitter AST). Language-
agnostic decision engines live in `internal/architecture/*`; per-language packs
adapt them in `internal/lang/<lang>/analyzers/*`.

## Build from source

```bash
go build -o scout-bin ./cmd/scout
```

Requires Go (see `go.mod`) and a C toolchain (CGO; scout links go-tree-sitter).
