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

### 💡 RSS feed presets = curated ExecutorInstallation configs
**Taxonomy:** IntegrationExecutor, ExecutorInstallation, SlotBinding.
"RSS presets" are **not** a template or artifact concern — feed URLs live in an
**ExecutorInstallation** config. So a preset = a curated installation config (a named set of
feed URLs) surfaced during slot binding in chat.
**Suggested sets (verify each URL):** Tech/startup (TechCrunch, The Verge, Ars Technica, HN),
Business (HBR, MIT Sloan, Reuters Business), Marketing/creator (Social Media Today, CMI),
Brazil/pt-BR (InfoMoney, Exame, Tecnoblog, Canaltech, Startupi).
**Next:** OpenSpec change once the authoring/seed mechanism (above) is decided.
**Links:** content-track backlog #5.

## UX

_(chat-first UX follow-ups tracked in `docs/superpowers/specs/2026-07-01-chat-first-ux-backlog.md`; migrate here as they graduate.)_

## Follow-ups / smaller

- Filter the integration selector to the param's required integration type (once >1 integration exists).
- Wire the classifier to tenant LLM budget controls before heavy production use.
- Generalize `BindingMatrixCard`'s input form to reuse `TemplateInputsForm`.
- `/inbox` should list only tasks (home is the chat-first entry).
