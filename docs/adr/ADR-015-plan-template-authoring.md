# ADR-015: PlanTemplate Authoring Model

> **⚠ Historical record — not the current source of truth.** The canonical description of the
> platform is the **[Platform Constitution](../architecture/harpia-platform.md)**, which
> supersedes ADR-001…017 as the reading order. See the
> **[ADR supersession map](README.md)** for how this ADR stands today. Where this ADR and the
> constitution disagree, the constitution wins.

**Status:** Accepted
**Date:** 2026-07-01
**Deciders:** thbertoldi

**References:**
- ADR-006: Domain-Driven Design for Agentic Architecture
- ADR-008: Tenant-Safe Boundaries for Generic Infrastructure
- ADR-012: Plan-Centric Task Model (PlanTemplate, PlanStep, ExecutorSKU, ArtifactType, snapshot invariant)

## Context

ADR-012 established the **PlanTemplate** as a reusable catalog DAG of **PlanSteps** —
"sold/configured as a product offering." Harpia's positioning is a *generic* AI-operations
platform whose **plan catalog is content, not hand-coded modules**.

Today, templates contradict that. The only template, `weekly-newsletter-linkedin`, is defined
across three SQL migrations:

- `database/migrations/000004_plan_tables.sql` — `plan_templates`, `plan_template_steps`, `plan_template_step_dependencies` inserts.
- `database/migrations/000011_plan_template_executor_metadata.sql` — `executor_requirement` + `default_executor_sku_key` per step.
- `database/migrations/000013_template_inputs_artifact_versions.sql` — the `input_parameters` JSONB, with runtime mappings.

Consequences of the status quo:

- Authoring a template requires an engineer to write SQL and cut a release. The catalog is
  effectively **schema**, not content.
- There is **no validation** at author time: a dangling DAG edge, a reference to a
  non-existent `ExecutorSKU` or `ArtifactType`, or an input `runtimeMapping` pointing at an
  unknown step is only discovered at runtime (or never).
- The classifier/chat can only ever propose the single seeded template, so the
  conversational proposal flow has nothing to choose between.

We need an authoring model that treats templates as versioned content and unblocks a real
catalog, without spending the Aiuna MVP runway (~2026-08-22) on infrastructure we do not yet need.

## Decision

### 1. MVP: declarative seed files (engineer-authored)

PlanTemplates are authored as **declarative YAML files**, one per template, under
`control-plane/internal/plans/templates/`, compiled into the binary with `go:embed`.

- **Schema stays in migrations; template *data* leaves migrations.** DDL for `plan_templates`,
  `plan_template_steps`, `plan_template_step_dependencies`, and the `input_parameters` column
  remains migration-owned. The row *content* moves to the declarative files.
- A file declares: template `key`, `name`, `description`, `vertical`, explicit `version`,
  ordered `steps` (`key`, `title`, `description`, `input_artifact_type`, `output_artifact_type`,
  `executor_requirement`, `default_executor_sku_key`), dependency `edges`, and `input_parameters`
  (with `runtimeMappings`) as defined in ADR-012 §2 and §4.

Authoring a template = write one validated YAML file + deploy. No migration.

### 2. Idempotent startup seeder

An `EnsurePlanTemplates` seeder runs at API startup, mirroring the existing
`EnsureCatalog` (executor SKUs) and `EnsureAgentTypes` pattern in `control-plane/cmd/api/main.go`.

- It **upserts by template `key`**, reconciling `plan_templates` and its child rows to match
  the declarative files (insert missing, update changed, and remove template rows/steps that
  no longer appear in any file).
- Templates remain **global** (not tenant-scoped), consistent with `Repository.ListTemplates`
  and `CopilotCatalog`.

### 3. Validation at load (the primary win over SQL)

The seeder validates every file before applying it and **fails fast** on:

- a malformed DAG — cycles, edges referencing unknown step keys, or unreachable steps;
- a step referencing an `ArtifactType` or `ExecutorSKU`/`executor_requirement` that does not exist;
- an `input_parameter` `runtimeMapping` whose target step/policy/seed field is unknown;
- a duplicate template `key` or step `key`.

Structural contract checks (output type of step N matches input type of step N+1, per ADR-012
invariants) are validated here too, so wiring errors surface at deploy, not mid-run.

### 4. Versioning and immutability

Each file carries an explicit integer `version`; the author bumps it on any change to the
template's contract. Runtime safety does **not** depend on this discipline: ADR-012 already
requires that a `PlanExecution` snapshots the `PlanConfiguration`, template version, behavior
policies, and executor installations. In-flight and scheduled runs therefore continue against
their snapshot even when a template file is edited and re-seeded.

### 5. Migrate the existing template

`weekly-newsletter-linkedin` is rewritten as the first declarative file, and its data inserts
in migrations `000004/000011/000013` are removed (DDL retained). Pre-v1, there is no
compatibility burden (see ADR positioning); we keep a **single authoring path** rather than
maintaining SQL and files in parallel.

### 6. Localized content unchanged

Template-facing copy continues to localize via flat i18n keys — `catalog.plan.<key>.name`,
`.description`, `catalog.plan.<key>.step.<step_key>.*`, and `plans.inputs.<param_key>.label`
in `frontend/src/lib/i18n/content/{en,pt-BR}.json`. The YAML `label`/`description` fields
remain the fallback when a key is absent.

### 7. Target end-state (declared, deferred): catalog service

The end-state is **C — a PlanTemplate catalog service**: CRUD APIs, server-side validation,
a publish lifecycle and versioning, an authoring UI, and Áreas-based RBAC — so non-engineers
(Harpia ops/product) author templates at runtime without a deploy. Cross-tenant / partner
authoring (marketplace) remains post-MVP, consistent with ADR-012's out-of-scope list.

The declarative-file schema in §1 is deliberately shaped to map onto that service's API
payload later: the file *is* the future create/update request body. Moving from B to C swaps
the loader (embedded files → API + persistence) without changing the template contract.

## Rationale

- **Content, not code.** Declarative data files honor the positioning that the catalog is
  content, and remove the migration-per-template friction, while staying git-versioned and reviewable.
- **Right-sized for the runway.** B is a small, well-understood change (a seeder + a schema),
  reusing an established seeding pattern. C is a real service; building it now would spend MVP
  time on authoring infrastructure with a single author (engineering) today.
- **Fail at deploy, not mid-run.** Load-time validation converts a class of silent runtime
  failures into fast, local errors.
- **No new runtime risk.** ADR-012's snapshot invariant already isolates runs from template
  edits, so a mutable-in-place seeded catalog is safe.

### Alternatives considered

| Alternative | Why not |
|---|---|
| **A — Keep SQL migrations** | Treats catalog as schema; no author-time validation; template needs a migration + release; contradicts "content, not coded modules." |
| **C — Catalog service now** | Correct end-state, but a large build (proto, service, versioning, authoring UI, RBAC) that spends MVP runway for a single (engineering) author today. Declared as the target instead. |
| **Dual path (SQL for existing, files for new)** | Two authoring mechanisms to maintain; contradicts this ADR the day it lands. Pre-v1 lets us migrate cleanly. |
| **JSON instead of YAML** | Matches existing JSONB, but worse authoring ergonomics for multi-step DAGs with comments. YAML files serialize to the same structures. |

## Consequences

### What becomes easier

- Authoring/iterating templates: one validated file + deploy, no SQL, no migration.
- Catch wiring errors (DAG, artifact/executor/mapping references) at deploy.
- The content track (more templates, RSS presets as installation configs) unblocks.
- A clean B→C path: the file is the future API payload.

### What becomes harder

- A startup seeder now reconciles catalog state; its upsert/removal logic must be correct and tested.
- Template authors must remember to bump `version` on contract changes (mitigated by snapshots).
- Feasibility remains bounded by available `ExecutorSKU`s (today: `rss-news-feed`,
  `linkedin-publish`, `newsletter-writer-senior`, `linkedin-voice-senior`); new *distinct*
  templates still need new executors.

### Implementation status

1. Accept ADR — Accepted 2026-07-01.
2. Add the YAML template schema + `go:embed` + `EnsurePlanTemplates` seeder with validation.
3. Migrate `weekly-newsletter-linkedin` to the first declarative file; remove its data inserts from migrations `000004/000011/000013` (retain DDL).
4. Author the content-track template library on this path.
5. (Deferred) C — catalog service + authoring UI + Áreas RBAC.

### Out of scope (post-MVP)

- Runtime authoring UI / catalog CRUD service (the C end-state).
- Tenant/partner-authored templates and a cross-tenant marketplace.
- Per-tenant template overrides or forks.
