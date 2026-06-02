# ai-guard

A local, deterministic scanner for AI-generated code.

ai-guard does **not** ship an LLM. It produces small, precise findings; your
coding agent (Claude Code, Codex, ...) reads them and fixes the code. Nothing
leaves your machine except dependency lookups (CVE data).

## Build

```bash
go build -o ai-guard ./cmd/ai-guard
```

## Use

```bash
./ai-guard scan              # scan current directory
./ai-guard scan --diff       # only files changed in git
./ai-guard scan --json .     # JSON report (for an agent to read)
./ai-guard context <target>  # cross-file context for a symbol or file
```

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
