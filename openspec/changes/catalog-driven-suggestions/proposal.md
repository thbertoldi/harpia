## Why

The home empty state shows four **hardcoded** suggestion chips
(`frontend/src/routes/+page.svelte:19-24`, keys `home.example.1..4` in
`i18n/content/{en,pt-BR}.json`). They are not derived from the `PlanTemplate` catalog, are not
shown inside a chat thread, and ignore the positioning that **the catalog is content**
(ADR-015). A reference design (Fable "Forge Agent") shows the target: catalog-shaped
suggestion chips that launch a full flow from the empty state. Harpia already has the data — a
server-streamed `ListPlanTemplates` RPC (`proto/harpia/plans/v1/plans.proto:13`) and localized
catalog content keys (`catalog.plan.<key>.*`) — but the chips don't use it.

## What Changes

- Replace the four hardcoded home suggestions with **catalog-derived** chips built from
  `ListPlanTemplates` (template name + a suggested prompt + a vertical-coded icon), localized
  via existing `catalog.plan.<key>.*` content keys with a new `catalog.plan.<key>.suggestion`
  prompt key.
- Show the same catalog-driven chips in the **bare-thread empty state** (the config-less
  branch of `frontend/src/routes/chat/[threadId]/+page.svelte`), so the in-thread empty state
  is also actionable.
- Selecting a chip reuses the existing path (`createThread` with the suggestion as the initial
  message → route to `/chat/[threadId]`), so the proposal flow is unchanged.
- Provide a small **generic fallback** set of prompts when the catalog is empty or loading, so
  tenants without templates aren't shown a blank state.
- All new copy via flat keys in `en` and `pt-BR`.

## Capabilities

### New Capabilities
- `catalog-driven-suggestions`: The home and bare-thread empty states render suggestion chips
  derived from the `PlanTemplate` catalog (`ListPlanTemplates`), with a generic fallback, that
  launch the chat thread flow.

## Impact

- Frontend: a small helper that loads templates (reuse the streamed `ListPlanTemplates`),
  derives a stable top-N suggestion list, and renders chips on the home page and the
  config-less thread branch; i18n content keys.
- No backend or proto changes (the RPC already exists).
- Tokens: chips use existing surface/border tokens + the energy accent on hover; inherits
  ADR-016 but does not block on it.
- Tests: frontend unit tests for the suggestion derivation (catalog → chips, fallback, locale
  resolution); the `hardcoded-copy` test enforces no literals.
