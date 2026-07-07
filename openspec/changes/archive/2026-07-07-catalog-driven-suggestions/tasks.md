## 1. Catalog Suggestion Helper

- [x] 1.1 Added `loadCatalogSuggestions(locale, max)` in `frontend/src/lib/plans/suggestions.ts`; consumes the new lighter `loadPlanTemplates(locale)` and returns a stable top-N `{ templateKey, iconKey, label, prompt }` list in catalog order.
- [x] 1.2 `label` resolves from the localized template name; `prompt` resolves from `catalog.plan.<key>.suggestion`, falling back to a composed `home.suggestion.compose` ("Create a {name}") string when the content key is absent.
- [x] 1.3 `verticalIconKey(vertical)` maps `vertical` → a stable icon key with an explicit `sparkles` default for unknown/empty verticals.
- [x] 1.4 Generic fallback (the existing `home.example.1..4`) is rendered by the `SuggestionChips` component when the catalog is empty/loading.
- [x] 1.5 Unit tests (`suggestions.test.ts`): vertical mapping (known/unknown/empty), explicit-suggestion resolution, composed fallback, locale-specific fallback, blank-name→key. 7 tests pass.

## 2. Home Empty State

- [x] 2.1 Replaced the hardcoded `exampleTasks` in `frontend/src/routes/+page.svelte` with `<SuggestionChips onSelect={handleSubmit} disabled={loading} />`; the generic fallback renders until the catalog stream resolves.
- [x] 2.2 Selection behavior unchanged: a chip calls the existing `createThread` → `/chat/[threadId]` path with the chip's prompt as the initial message.
- [x] 2.3 Uses existing surface/border tokens + the energy accent for icons; no raw literals.

## 3. Bare-Thread Empty State

- [x] 3.1 Rendered `SuggestionChips` in the config-less branch of `frontend/src/routes/chat/[threadId]/+page.svelte` when the thread has no messages.
- [x] 3.2 Selecting a chip seeds the current thread via `appendThreadMessage(... "OVERSEER", "USER_TEXT" ...)` (the same path the composer uses); the auto-propose effect then fires once the message arrives.

## 4. Localization

- [x] 4.1 Added `catalog.plan.weekly-newsletter-linkedin.suggestion` to `frontend/src/lib/i18n/content/{en,pt-BR}.json`.
- [x] 4.2 Added `home.suggestion.compose` to `frontend/src/lib/i18n/{en,pt-BR}.json`; the `hardcoded-copy` test passes.

## 5. Verification

- [x] 5.1 Run `openspec validate --changes catalog-driven-suggestions` (passes strict).
- [x] 5.2 `cd frontend && bunx vitest run` — 393 tests pass (incl. 7 new `suggestions` tests).
- [x] 5.3 `cd frontend && bun run check` — 12 errors / 5 warnings (the known baseline; no new diagnostics) and `bun run lint` clean.
- [ ] 5.4 Smoke: with a seeded catalog, home and an empty thread show catalog chips; with an empty catalog, the generic fallback appears and selecting either creates/seeds a thread.
