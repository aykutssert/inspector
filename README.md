# inspector

A deterministic scanner for AI-generated code: security, bugs, performance, and
cross-file context.

inspector does **not** ship an LLM. It produces small, precise findings; your
coding agent (Claude Code, Codex, ...) reads them and fixes the code. Detection
leans on proven tools, so it pulls rule packs and CVE data as needed — normal
developer tooling, not your source code going to a cloud model.

## Build

```bash
go build -o inspector ./cmd/inspector
```

## Use

```bash
./inspector scan              # scan current directory
./inspector scan --diff       # only files changed in git
./inspector context <target>  # cross-file context for a symbol or file
```

`scan` prints human-readable output on a terminal and JSON when piped (so an
agent can read it) — no flag to remember. A missing scanner is reported as an
error, never a silent pass: a clean report means the tools actually ran.

`context` is the core feature: it gives an agent the definitions, callers, and
imports it needs to change code without breaking the rest of the project.

## How it works

```
        core (scan orchestrator)
                  |
  semgrep   osv   tree-sitter   git-log
  (rules)  (CVE)    (graph)    (history)
```

- Detection delegates to proven tools (`semgrep`, `osv-scanner`).
- The context graph is built in-process from a tree-sitter parse.
- Add an analyzer or a language without touching the core (plugin design).

External tools are optional and auto-skipped if missing.

First language: **JS/TS**. More to come.
