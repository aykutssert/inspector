# inspector

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