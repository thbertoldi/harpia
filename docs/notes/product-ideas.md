# Product Ideas Log

Living inbox + roadmap index for Harpia. Every idea lands here first, tagged with
[taxonomy](../adr/ADR-012-plan-centric-task-model.md) terms, then graduates:

> idea → **log** (here) → **ADR** (`docs/adr/`) if it's an architecture decision → **OpenSpec change** (`openspec/`) when ready to build.

This file is tool-neutral (read by Codex, OpenCode, Cursor, Claude Code). See
[AGENTS.md](../../AGENTS.md) for how it fits the workflow.

**Status legend:** 💡 idea · 🔬 needs decision (→ ADR) · 🛠️ building (→ OpenSpec) · ✅ shipped

---

## Vision / Architecture

### 🛠️ Fully conversational plan configuration → RUNNABLE
**Taxonomy:** PlanConfiguration, SlotBinding, OverseerBinding, PlanBehaviorPolicies, PlanSchedule.
The entire configuration — slot bindings (executors), overseer assignment, behavior
policies, and schedule — should be completed **in the chat interface**, so the plan is
**RUNNABLE the moment planning finishes**. This is ADR-012 §10 ("chat is a
PlanConfiguration assistant") made real end-to-end.
**Current state:** only the proposal/confirmation step is conversational (shipped via
`conversational-plan-proposal`). The configured side has a backend state machine
(`planassistant`: `BINDING_MATRIX → landing`) but binding is a flat matrix, overseer is a
read-only stub, policies are LinkedIn-only, and schedule is a post-save modal.

**Architecture:** extend the `planassistant` backend state machine (persisted `ASSISTANT_PROMPT`
messages) — NOT client-side like the proposal. Re-enable the already-defined `USER_SELECTION`
and `STEP_REBOUND` message kinds. No new ADR (ADR-012 §10 covers it).

**UX decision:** conversational-first, with the **binding matrix kept as a collapsible
"edit all" fallback**; both surfaces sync via `UpdatePlanConfiguration`.

**Decomposition (4 sequenced OpenSpec changes, 1→2→3→4):**
1. ✅ `conversational-slot-binding` — guided per-step "who handles X?" chips (incl. RSS presets); re-enable USER_SELECTION/STEP_REBOUND.
2. ✅ `conversational-overseer` — "who oversees the agent steps?" (wire the stubbed overseer; MVP self/tenant user).
3. `conversational-policies` — approval mode + elicitation timeout as enum chips (generalize beyond LinkedIn).
4. `conversational-schedule-promote` — schedule in-flow, validate invariants, promote → RUNNABLE (reuse the shipped celebration), add "Revise" backward handoff.

**Links:** ADR-012 §10; builds on `openspec/changes/archive/2026-07-01-conversational-plan-proposal`; shipped slices:
`openspec/changes/archive/2026-07-01-conversational-slot-binding`,
`openspec/changes/archive/2026-07-01-conversational-overseer`.

## Content

### ✅ Are PlanTemplates content or coded modules? → decided (ADR-015)
**Taxonomy:** PlanTemplate, PlanStep.
**Decision:** MVP = **B, declarative YAML seed files** (engineer-authored, `go:embed` +
idempotent `EnsurePlanTemplates` seeder with load-time validation); migrate the existing
SQL-seeded template onto this path. **Target = C, a catalog service** (runtime authoring UI +
Áreas RBAC), deferred. See **[ADR-015](../adr/ADR-015-plan-template-authoring.md)**.
**Constraint:** only 4 ExecutorSKUs exist today (`rss-news-feed`, `linkedin-publish`,
`newsletter-writer-senior`, `linkedin-voice-senior`) — new *distinct* templates need new
executors; otherwise we can only recombine these four.
**Next:** OpenSpec change to implement B (schema + seeder + migrate existing template), then the template library.

### ✅ RSS feed presets = curated ExecutorInstallation configs → shipped
**Taxonomy:** IntegrationExecutor, ExecutorInstallation, SlotBinding.
Feed URLs live in an **ExecutorInstallation** config, not in templates/artifacts. Shipped as
seeded named `rss-news-feed` installations (`EnsureTenantRSSPresetInstallations`), 4 groups
(Tech, Business, Marketing, Brazil/pt-BR), each URL verified live; HBR/Reuters/CMI dropped
(discontinued RSS) — see design.md. **Links:** `openspec/changes/archive/2026-07-01-plan-template-authoring`.

### ✅ First expanded template: news-digest-draft → shipped
Proved the multi-template declarative path (fetch-news → write-draft → TextDraft, review-only).
A richer library is **blocked on new ExecutorSKUs** (blog/email/X integrations don't exist yet).

## Execution

### 🛠️ Milestone E: a configured plan actually runs (and on schedule)
**Taxonomy:** PlanConfiguration, PlanExecution, StepExecution, Artifact, SlotBinding,
SeedArtifact, PlanBehaviorPolicies, PlanSchedule, runtimeMappings.
The Temporal engine, worker, scheduling (real Temporal Schedules), RSS + LinkedIn handlers,
the Python agent activity (two shipped manifests), and RUNNABLE validation already work. The
gap is upstream: **nothing on the server expands a template's `runtimeMappings` +
`parameter_values` into the config's seeds / slot bindings / behavior policies** — it is faked
per-template in the frontend (`materializeLinkedInInputValues`), so a generic chat-created
config has empty seeds and can't run. Also missing: execution-time resolution of the rolling
`date_range` preset (`last_7_days`) and pre-flight seed validation. Decomposed into a
server-authoritative materializer, per-run date resolution, pre-flight readiness, agent-worker
deployment, and an end-to-end run of `weekly-newsletter-linkedin`. Generic agent executor
(beyond the two manifests) is a deliberate follow-up.
**Links:** `openspec/changes/plan-execution-runtime`.

## UX

### 🛠️ Chat-first plan lifecycle experience
**Taxonomy:** PlanTemplate, PlanConfiguration, PlanExecution, Artifact, OverseerBinding,
PlanSchedule.
The UX north star is that the user **talks to Harpia**, Harpia identifies and configures
plans, then keeps execution, schedule, control, audit, artifacts, and later edits available
through the same conversational mental model. Chat must not feel like a launcher for modal
forms. Plan proposal, refinement, configuration, post-create transition, run/schedule, and
"anything else?" should accumulate as a conversation.

**Captured direction:** choose the rich conversational refinement direction ("C"):
best-match plan highlighting, recommended themes, audience, topics to avoid, multiple
source groups, clear date-range explanation, and a natural final confirmation. After the
plan is configured, keep the user in the thread with actions like Run now / Schedule /
Review plan / Adjust configuration / Ask about another plan.

**Cohesion requirement:** the same PlanConfiguration must look and behave like the same
object across chat, plan lists, canvas, settings/configuration, execution detail, artifact
views, and audit surfaces. Structured pages may be denser than chat, but they must reuse
the same vocabulary and controls.

**Links:** detailed backlog and investigation notes in
`docs/superpowers/specs/2026-07-01-chat-first-ux-backlog.md`.

### 🛠️ Fable-inspired execution + artifact chat UX
**Taxonomy:** PlanExecution, StepExecution, Artifact, ArtifactType, PlanTemplate (catalog).
A reference design (Fable "Forge Agent" mock) demonstrates the experience we want: a
**collapsible live plan/step card** with progress + connector timeline, an **artifact preview
slide-over** (preview/code tabs, version, copy, sandboxed HTML/markdown) launched from an
inline artifact card, **catalog-driven suggestion chips**, and a live header **status pill**.
Harpia already has the data — `STEP_STARTED`/`STEP_BOUND`(=step completed)/`RUN_*` events,
`ARTIFACT_CREATED/UPDATED`, the `PlanTemplate` catalog (`ListPlanTemplates`), and a 5-layer
surface token system — but renders them as flat `SystemEventCard` lines with only
text/list/json preview renderers and a persistent rail, not the rich transient UX.

**Direction decided:** adapt the *patterns* (not the violet palette) to our tokens; keep
event-based streaming (no token stream). Visual refresh → **[ADR-016](../adr/ADR-016-design-token-refresh.md)**
(Harpy Eclipse: gold identity + electric-teal energy accent, cooler obsidian, status tokens).

**Decomposition (3 OpenSpec changes, ADR-016 first):**
1. 🛠️ `live-execution-chat` — collapsible `PlanExecutionCard` (step status timeline, progress
   bar), header "Executing…/Ready" pill, generating/typing polish. Frontend-only; reuses
   existing events.
2. 🛠️ `artifact-preview-panel` — transient slide-over (preview/code tabs, version, copy,
   reload) + renderer registry (add `html`/`markdown`/`image`; keep `text/json/list`); inline
   `ArtifactCard` message launching it. Adds proto preview variants.
3. 🛠️ `catalog-driven-suggestions` — replace the 4 hardcoded home chips with
   `PlanTemplate`-catalog-derived suggestions, also shown in the bare-thread empty state.
   Frontend-only; reuses `ListPlanTemplates`.

**Links:** ADR-016; `openspec/changes/{live-execution-chat,artifact-preview-panel,catalog-driven-suggestions}`.

### 💡 Proactive memory capture from conversational configuration
**Taxonomy:** PlanConfiguration, PlanBehaviorPolicies, MemoryResource, MemoryBinding,
Artifact.
When a user gives reusable preferences while configuring or executing a plan in chat
(for example LinkedIn writing preferences such as tone, audience, language, topics to
avoid, or brand voice), Harpia should proactively offer to save those preferences as an
explicit reusable memory. Future configurations of the same or related PlanTemplate can
then suggest defaults from that saved memory to prefill fields and explain why those
defaults were chosen.

**Boundary:** this must follow ADR-014. Preferences are not hidden agent memory and must not
silently accumulate from elicitation answers. Saving should be a visible promotion flow
that creates or updates a tenant/workspace/user-scoped `MemoryResource` with provenance
back to the originating PlanConfiguration / PlanExecution / chat turn, and later use should
flow through explicit `MemoryBinding` or an equivalent inspectable configuration object.

**Product shape:** during configuration, the assistant can ask "Save these as your
LinkedIn writing preferences?" after the user finalizes relevant fields. On future runs,
the assistant can say it found saved preferences and offer chips to apply, revise, or
ignore them before materializing parameter values.

## Follow-ups / smaller

- Add a test that validates the **real embedded template YAML** (seeder tests currently only use synthetic YAML → a typo ships and fails only at API boot).
- Filter the integration selector to the param's required integration type (once >1 integration exists).
- Wire the classifier to tenant LLM budget controls before heavy production use.
- Generalize `BindingMatrixCard`'s input form to reuse `TemplateInputsForm`.
- `/inbox` should list only tasks (home is the chat-first entry).
