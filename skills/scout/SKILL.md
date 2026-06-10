---
name: scout
description: Use when finishing a feature, fixing a bug, before committing code, or when the user types `/scout`, asks to scan, check health, or triage the codebase.
version: "1.0.0"
---

# Scout

Scans multi-language codebases for security, performance, correctness, and architecture issues, producing a 0–100 health score.

## Trigger Conditions
- User types `/scout`
- User asks to "run scout", "scan codebase", "audit project", or "check health score"
- Before committing code or finalizing changes.

## Workflow

1. **Environment Check**:
   First, run `scout doctor --json` to diagnose the local workspace.
   Parse the JSON output.

2. **Resolve Dependencies**:
   - If any `system` dependency with status `ERROR` is found (e.g. `node`, `git` missing):
     Ask the user to install it manually.
   - If any `system` dependency with status `WARNING` is found (e.g. `semgrep`, `oxlint`, `osv-scanner` missing):
     Ask the user for permission to install it (e.g., `npm i -g oxlint` or `pip install semgrep`), then run the installation command.
   - If any `toolchain` dependency with status `WARNING` is found (e.g. `knip`, `typescript`, `svelte`, `tailwind` missing node_modules):
     Run `npm install` inside the reported toolchain path (provided in the JSON `path` or `install_hint` fields).

3. **Run Scan**:
   Once critical/optional tools are resolved, run the scan:
   - For general scan: `scout scan`
   - For regression/diff check (only changed files): `scout scan --diff`

4. **Verify & Triage**:
   - Print the Health Score (0-100) prominently to the user.
   - Review findings by severity (errors first, then warnings).
   - Each finding includes: `rule_id`, `severity`, `confidence`, `file`, `line`, `message`, `fix`, and `snippet` (the relevant lines of code). Use `snippet` directly — no need to re-read the source file.
   - If a finding needs more context (why it matters, bad/good code examples), run:
     `scout explain <rule_id>`
     This returns `why`, `bad`, `good`, and `fix` fields. Use this before attempting a fix on any non-obvious rule.
   - `confidence: rule` = deterministic, act on it. `confidence: hint` = heuristic, verify against the snippet before acting.
