---
name: harpia
description: Use when working in the Harpia repository. Routes agents to the canonical project guide, Platform Constitution, OpenSpec change artifacts, current delivery direction, and verification gates without duplicating mutable architecture.
---

# Harpia

This skill is a router, not an architecture source. Do not infer current Harpia behavior
from this file.

## Load canonical context

1. Read repository-root `AGENTS.md` before doing any work. It is authoritative for working
   conventions, delivery constraints, terminology routing, and verification gates.
2. Before modeling product behavior, taxonomy, bounded contexts, data ownership, or system
   boundaries, read `docs/architecture/harpia-platform.md`. The Platform Constitution is
   the current architecture source of truth and wins over conflicting ADR history.
3. Load `docs/architecture/mvp-roadmap.md` for current build direction and
   `docs/architecture/cleanup-backlog.md` when the task intersects known drift or cleanup.
4. Read the relevant `openspec/specs/` capability and active `openspec/changes/<name>/`
   proposal, design, specs, and tasks before planning or implementing a build-ready change.
5. Read individual files under `docs/adr/` only for task-relevant historical rationale or
   decisions not yet folded into the Constitution.

Load only the context relevant to the request. If the code, an ADR, or another document
conflicts with the Constitution, report the mismatch rather than silently choosing the
older model.

## Route changes through the repository workflow

- Capture an uncommitted product idea in `docs/notes/product-ideas.md`.
- Record an architecture decision in an ADR, then fold accepted current truth into the
  Platform Constitution.
- Plan and track build-ready code changes through OpenSpec: propose → design/specs/tasks →
  apply → validate → archive.
- Do not create BMAD, Superpowers, or parallel planning artifacts for Harpia changes.

## Work within the repository surfaces

- Go control plane: `control-plane/`
- Python agent runtime: `agent-runtime/`
- Svelte frontend: `frontend/`
- Contracts: `proto/`
- Database and deployment: `database/`, `deploy/`
- Catalog agent manifests: `agents/`

Inspect surrounding code and the active OpenSpec change before choosing files or patterns.
Preserve unrelated work already present in the tree.

## Finish against the current gates

Run every applicable whole-surface linter, type check, test, generation step, and baseline
rule listed in `AGENTS.md`, plus focused tests from the active OpenSpec task. Do not copy a
fixed command list from this skill: `AGENTS.md` is deliberately the single maintained
source. Keep OpenSpec task checkboxes accurate and do not claim completion while a required
gate or mandatory review remains outstanding.
