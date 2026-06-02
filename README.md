# ai-guard (Inspector)

Deterministic, local code security/quality scanner for AI-generated code.

We do **not** ship an LLM. The intelligence is the user's existing coding-agent
subscription (Claude Code, Codex, OpenCode, ...). ai-guard produces small,
relevant, deterministic findings; the agent's LLM reads them and fixes the code.

See [project.md](project.md) for the full design, decisions, and roadmap.

## Build

```bash
go build -o ai-guard ./cmd/ai-guard
```

## Use

```bash
./ai-guard scan              # full scan of current dir
./ai-guard scan --diff       # only locally changed files (git)
./ai-guard scan --json .     # JSON report (what an agent harness reads)
```

## Architecture

```
        [core: scan orchestrator]
                    │
     ┌──────┬───────┼────────┬──────────┐
   semgrep  osv   tree-sitter git-log   <new>
  (rules)  (CVE)   (AST)    (history)  (plugin)
```

- Analyzers implement `core.Analyzer` — add one, orchestrator untouched.
- Languages implement `core.LanguageAdapter` — add one, core untouched.
- Detection delegates to proven tools (`semgrep`, `osv-scanner`); we orchestrate.

## External tools (optional, auto-skipped if missing)

- `semgrep` — static security/quality rules
- `osv-scanner` — dependency CVE scanning
- `git` — local history risk analysis (no GitHub connection needed)

First target language: **JS/TS**. Multi-language by design.
