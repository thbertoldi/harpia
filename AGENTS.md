# Harpia — Agent Guide

**This file is the shared source of truth for every coding agent on this repo**
(Codex, OpenCode, Cursor, Claude Code). Keep it tool-neutral. Tool-specific
entry files (e.g. `CLAUDE.md`) must import this, not duplicate it.

Harpia is a chat-first, AI-enabled **operations platform** — a *generic* platform,
**not** a vertical CRM/ERP. The plan catalog is **content**, not hand-coded modules.

> **Architecture & taxonomy source of truth:** the **[Platform Constitution](docs/architecture/harpia-platform.md)**.
> It supersedes ADR-001…017 as the reading order (the ADRs remain as dated history).
> Read the constitution before modeling anything; the **[MVP Roadmap](docs/architecture/mvp-roadmap.md)**
> and **[Cleanup Backlog](docs/architecture/cleanup-backlog.md)** are its companions.
> This file (AGENTS.md) remains the source of truth for *how we work* — delivery
> constraints, workflow, and verification gates below.

## Ubiquitous language (use these terms exactly)

The domain taxonomy is authoritative. The reconciled glossary lives in the
**[Platform Constitution §4](docs/architecture/harpia-platform.md#4-ubiquitous-language-the-reconciled-glossary)** —
read it before modeling anything. The ADRs below are retained as historical rationale, but
where they disagree with the constitution, **the constitution wins**:
- **[ADR-012 Plan-Centric Task Model](docs/adr/ADR-012-plan-centric-task-model.md)** — the core taxonomy (as amended by ADR-015/017).
- **[ADR-006 Domain-Driven Design](docs/adr/ADR-006-domain-driven-design.md)** — bounded contexts (now six; see constitution §5).
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
- A **conversation is a PlanConfiguration assistant** that may spawn several plans (1:N;
  constitution §9), not a generic task creator.

Corollaries that constrain design:
- Business state lives as typed **Artifacts** and **Resources**; UI surfaces are
  **browsers/editors over that model**, not bespoke CRUD stores (constitution §2, §8).
- Business **Áreas are RBAC entities** (membership-based; constitution §11): memberships gate
  Plans/Agents/Integrations/Artifacts/Resources.
- Aiuna is the **AI-operations layer over a downstream system of record** (Odoo first);
  plans operate downstream as governed, reversible operations (constitution §15).
- Feed URLs, OAuth, downstream endpoints/credentials, etc. belong to an **ExecutorInstallation
  config**, not to templates or artifacts.

## How we work — idea → decision → build

Ideas arrive at different altitudes; route each to the right home so nothing is lost:

1. **Idea log** — capture every idea in **[docs/notes/product-ideas.md](docs/notes/product-ideas.md)**,
   themed and tagged with taxonomy terms. This is the inbox + roadmap index.
2. **Fold into the [Platform Constitution](docs/architecture/harpia-platform.md)** — when an
   idea is an architecture *decision*, record it directly in the constitution (the single
   source of truth). **We no longer create new ADRs** — they reintroduce the fuzzy-context
   problem the constitution removed; ADR-001…018 are frozen history under their banners.
3. **OpenSpec change** (`openspec/`) — when an idea is ready to build. Use the
   `openspec` CLI (v1.5+) or the `opsx-*` commands. Flow: propose → design → specs → tasks → apply → archive.
4. Each log entry links forward to its constitution section / OpenSpec change so the trail is traceable.

## Delivery constraints (non-negotiable)

- **Trunk-based**, small logical commits. Never add a `Co-Authored-By: Claude` trailer.
- **Pre-v1**: break schemas/routes/protocols freely — no migrations, deprecation shims, or transitional redirects until v1 is cut.
- **Fonts are LOCKED; color/depth is governed** by the Harpy Eclipse token system
  (constitution §10) — change tokens only through the system, never ad hoc. Animate
  layout/opacity/transform only.
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

**Linters MUST pass before any work is considered finished** (mirrors CI). Run
the linters for every surface you touched — formatting/format regressions on
files you did not author still fail CI, so run the whole-project lint, not just
changed files:

- Frontend lint: `cd frontend && bun run lint` (`prettier --check . && eslint .`).
- Python lint: `cd agent-runtime && ruff check src/`.
- Helm lint: `mise run helm-lint` (or `helm lint deploy/**/`).
- Frontend tests: `cd frontend && bunx vitest run <file>` (NOT `bun test`).
- Frontend types: `cd frontend && bun run check` — **12 pre-existing baseline errors**;
  introduce no new ones.
- Backend: `cd control-plane && go test ./...`
- Acceptance: `mise run acceptance` is required before delivery/archive; unchecked acceptance means the change is not done.
- Proto: `cd proto && buf lint`
- Run the app (single command): `mise run dev` (Tilt; DeepSeek key in `deploy/dev/kind/secrets.local.env`).

## Active direction

- **Epic — fully conversational configuration:** a **conversation** is a PlanConfiguration
  assistant that may produce **several** plans (constitution §9); the entire configuration
  (slot bindings, overseer, behavior policies, schedule) should be completed in chat, ending
  in a **RUNNABLE** plan. Today only the proposal/confirmation step is conversational;
  binding still happens on the matrix/configured surface. See the idea log for status.
- **Current build target:** the [MVP Roadmap](docs/architecture/mvp-roadmap.md) —
  content-depth-first (Resource layer → browsers → rich content). Phase 0 cleanup is in the
  [backlog](docs/architecture/cleanup-backlog.md).
