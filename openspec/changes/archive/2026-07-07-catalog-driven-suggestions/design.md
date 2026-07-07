## Context

The home page builds its suggestion chips from four fixed localized strings
(`frontend/src/routes/+page.svelte:19-24` → `home.example.1..4` in
`i18n/content/{en,pt-BR}.json`). They never reach the chat thread. Meanwhile the catalog is
real and queryable: `ListPlanTemplates` (`proto/harpia/plans/v1/plans.proto:13`) streams
templates, each with localized content keys (`catalog.plan.<key>.name/.description` and step
copy, per ADR-015). The classifier/proposal flow already turns a free-text prompt into a
`PLAN_PROPOSED`, so a chip's job is simply to seed `createThread` with good initial text.

So the gap is: the empty states don't reflect what the tenant can actually do.

## Goals / Non-Goals

**Goals:**
- Derive home and bare-thread empty-state suggestion chips from the live `PlanTemplate` catalog.
- Keep selection behavior identical to today (create thread → `/chat/[threadId]`).
- Localize chip label + prompt via existing catalog content keys (add one `suggestion` prompt
  key per template).
- Fall back to a small generic set when the catalog is empty/loading.

**Non-Goals:**
- Personalized ranking / ML suggestions (stable top-N by template order is enough for MVP).
- Suggestion analytics / A-B ranking.
- Changing the proposal/classification backend.
- Surfacing suggestions mid-conversation (only empty states).

## Decisions

1. **Catalog is the single source; chips map 1:1 to templates.**

   A helper `loadCatalogSuggestions(locale)` consumes the streamed `ListPlanTemplates`, takes a
   stable top-N (template catalog order, capped — e.g. 4–6), and returns `{ templateKey, icon,
   label, prompt }`. `label` = `catalog.plan.<key>.name`; `prompt` = a new
   `catalog.plan.<key>.suggestion` content key (a ready-to-send prompt), falling back to a
   composed "Create a {name}" string.

   Alternative considered: derive the prompt by reusing `.description`. Rejected: descriptions
   are explanatory, not send-ready; a dedicated `suggestion` prompt key is clearer and
   translatable.

2. **Stable, non-personalized ordering.**

   Order chips by the catalog's own order (the seeded template order from ADR-015's
   `EnsurePlanTemplates`). No popularity/recency signals now — add later if useful.

   Alternative considered: rank by usage. Rejected: over-engineering for the current
   single-digit catalog; no usage signal plumbed to the frontend today.

3. **Bare-thread empty state reuses the same chips.**

   The config-less branch (`+page.svelte` config-less path) renders the same chip component
   above the composer when there are no messages, so an empty thread is immediately
   actionable. Selecting a chip calls the same `createThread`/append path used on home.

   Alternative considered: keep suggestions home-only. Rejected: the user-chosen direction
   wants in-thread actionability, matching the Fable reference.

4. **Generic fallback when the catalog is empty/loading.**

   If `ListPlanTemplates` returns nothing (or while loading), show a small fixed set of generic
   prompts (e.g. "Summarize a topic", "Draft a LinkedIn post", "Round up recent news"),
   localized under `home.example.fallback.*`. This replaces today's hardcoded four only when
   the catalog is unavailable.

   Alternative considered: show nothing while loading. Rejected: an empty state must never be
   blank.

5. **Localization and tokens.**

   Chip label + prompt via flat content keys; vertical-coded icons map deterministically from
   template `vertical`. Chips use existing surface/border tokens and the energy accent on
   hover/focus (ADR-016). No raw literals.

## Risks / Trade-offs

- **Catalog latency on first paint.** `ListPlanTemplates` is streamed; chips may pop in.
  Mitigation: render the generic fallback immediately, swap to catalog chips when the stream
  resolves; keep the count stable to avoid layout shift.
- **Prompt quality per template.** A bad `suggestion` prompt yields a poor proposal.
  Mitigation: author the `suggestion` key alongside each template's content in the declarative
  YAML content track (ADR-015); review during content authoring.
- **Vertical→icon mapping drift.** A new vertical without an icon falls back to a default.
  Mitigation: explicit default in the icon map; tested.

## Migration Plan

Pre-v1; forward-only, frontend-only:
1. Add the suggestion helper + unit tests (catalog→chips, fallback, locale).
2. Add `catalog.plan.<key>.suggestion` content keys for existing templates + fallback keys in
   `en`/`pt-BR`.
3. Render chips on home and the bare-thread branch; remove the hardcoded `home.example.1..4`
   usage (retain keys as the fallback source if desired, or migrate).
4. Lint/typecheck/test gates.
