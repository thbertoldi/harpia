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
`conversational-plan-proposal`). Slot binding / overseer / policies / schedule still
happen on the binding-matrix / configured surface.
**Next:** decompose into stages (bind → overseer → policies → schedule → confirm RUNNABLE),
each its own OpenSpec change. Likely a multi-change epic.
**Links:** ADR-012 §10; builds on `openspec/changes/conversational-plan-proposal` (shipped).

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

## UX

_(chat-first UX follow-ups tracked in `docs/superpowers/specs/2026-07-01-chat-first-ux-backlog.md`; migrate here as they graduate.)_

## Follow-ups / smaller

- Add a test that validates the **real embedded template YAML** (seeder tests currently only use synthetic YAML → a typo ships and fails only at API boot).
- Filter the integration selector to the param's required integration type (once >1 integration exists).
- Wire the classifier to tenant LLM budget controls before heavy production use.
- Generalize `BindingMatrixCard`'s input form to reuse `TemplateInputsForm`.
- `/inbox` should list only tasks (home is the chat-first entry).
