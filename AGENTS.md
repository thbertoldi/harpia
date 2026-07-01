# Harpia — Agent Guide

**This file is the shared source of truth for every coding agent on this repo**
(Codex, OpenCode, Cursor, Claude Code). Keep it tool-neutral. Tool-specific
entry files (e.g. `CLAUDE.md`) must import this, not duplicate it.

Harpia is a chat-first, AI-enabled **operations platform** — a *generic* platform,
**not** a vertical CRM/ERP. The plan catalog is **content**, not hand-coded modules.

## Ubiquitous language (use these terms exactly)

The domain taxonomy is authoritative. Read before modeling anything:
- **[ADR-012 Plan-Centric Task Model](docs/adr/ADR-012-plan-centric-task-model.md)** — the core taxonomy.
- **[ADR-006 Domain-Driven Design](docs/adr/ADR-006-domain-driven-design.md)** — bounded contexts.
- **[ADR-008 Tenant-Safe Boundaries](docs/adr/ADR-008-tenant-safe-generic-infra-boundaries.md)** — generic-infra isolation.

Key terms (see ADR-012 for full definitions):
- **PlanTemplate** — reusable catalog DAG of **PlanSteps**. A PlanStep is a contract
  `(input_artifact_type → output_artifact_type)` — the *what*, independent of the *who*.
- **Executor** fills a slot: **ExecutorSKU** (sold) → **ExecutorEntitlement** (granted)
  → **ExecutorInstallation** (tenant-configured; carries OAuth/feed config).
  A **SlotBinding** points a step at an *installation*, never a global SKU.
- **PlanConfiguration** — tenant binding (slot bindings + **OverseerBindings** +
  **PlanBehaviorPolicies** + seed inputs + **PlanSchedule**); lifecycle
  `DRAFT → RUNNABLE → SCHEDULED → DISABLED → ARCHIVED`.
- **Artifact / ArtifactType** — typed payloads flowing between steps.
- **PlanExecution / StepExecution** — one run of a configuration walking the DAG.
- The **chat interface is a PlanConfiguration assistant** (ADR-012 §10), not a generic task creator.

Corollaries that constrain design:
- Plans generate typed **Artifacts**; UI surfaces are **browsers over the Artifact stream** — no out-of-band CRUD.
- Business **Áreas are RBAC entities**: memberships gate Plans/Agents/Integrations/Artifacts.
- Feed URLs, OAuth, etc. belong to an **ExecutorInstallation config**, not to templates or artifacts.

## How we work — idea → decision → build

Ideas arrive at different altitudes; route each to the right home so nothing is lost:

1. **Idea log** — capture every idea in **[docs/notes/product-ideas.md](docs/notes/product-ideas.md)**,
   themed and tagged with taxonomy terms. This is the inbox + roadmap index.
2. **ADR** (`docs/adr/`) — when an idea is an architecture *decision*.
3. **OpenSpec change** (`openspec/`) — when an idea is ready to build. Use the
   `openspec` CLI (v1.5+) or the `opsx-*` commands. Flow: propose → design → specs → tasks → apply → archive.
4. Each log entry links forward to its ADR / OpenSpec change so the trail is traceable.

## Delivery constraints (non-negotiable)

- **Trunk-based**, small logical commits. Never add a `Co-Authored-By: Claude` trailer.
- **Pre-v1**: break schemas/routes/protocols freely — no migrations, deprecation shims, or transitional redirects until v1 is cut.
- **Design tokens (colors, fonts) are LOCKED.** Animate layout/opacity/transform only.
- **All user-facing copy needs `en` + `pt-BR`** via flat `translate()` keys in `frontend/src/lib/i18n/{en,pt-BR}.json`.
  Template/catalog content localizes via `catalog.plan.<key>.*` and `plans.inputs.<key>.label`.
- **Protect the domain**: keep infrastructure/deps out of domain packages (hexagonal / ports & adapters).
- Highest quality bar. Value decisions are the human's; don't relitigate them as "overengineering."

## Execution model (who writes the code)

Planning and review happen with the strong model; **implementation is delegated to
cheaper executors — Codex (GPT 5.5 xhigh) and Cursor (Composer 2.5)** — spread across
the agent roster to manage quota/cost. Therefore: **plans/briefs must be executor-cold**
— self-contained, no assumed conversation context, with explicit file paths and
per-task verification.

## Verification gates

- Frontend tests: `cd frontend && bunx vitest run <file>` (NOT `bun test`).
- Frontend types: `cd frontend && bun run check` — **12 pre-existing baseline errors**;
  introduce no new ones.
- Backend: `cd control-plane && go test ./...`
- Proto: `cd proto && buf lint`
- Run the app (single command): `mise run dev` (Tilt; DeepSeek key in `deploy/dev/kind/secrets.local.env`).

## Active direction

- **Epic — fully conversational configuration:** the entire PlanConfiguration
  (slot bindings, overseer, behavior policies, schedule) should be completed in chat,
  ending in a **RUNNABLE** plan (ADR-012 §10). Today only the proposal/confirmation
  step is conversational; binding still happens on the matrix/configured surface.
  See the idea log for status.
