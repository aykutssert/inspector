---
name: scout-context
description: Use when you need a map of the whole project before working in it, or when the user types `/scout-context`, asks "give me the project structure", "how is this repo organized", or "map the codebase".
version: "0.1.0"
---

# Scout Context

Produces a structural map of the **entire project** — its modules, entry points,
key files, and how they connect — so the agent understands the codebase shape
before editing. Deterministic AST/import graph, not a whole-file dump.

> Status: early. The CLI surface is still being built out; treat the output as a
> high-level map, not a complete cross-reference yet.

## Trigger Conditions
- User types `/scout-context`
- User asks "map this repo", "show the project structure", "how is this organized"
- Before starting work in an unfamiliar codebase.

## Workflow

1. **Run** `scout context` at the repo root (add `--root <dir>` if needed). It
   emits the project structure (JSON when piped). If `scout` is missing, run
   `scout doctor` first and install it per the scout skill.

2. **Use the map**: orient from the entry points and module boundaries it
   reports, then read only the files relevant to the task. Do not load the whole
   repo into context — the map is there so you can pick the right slice.
