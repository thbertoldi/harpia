# M6 Lapidação — Verification Log

**Date:** 2026-06-25
**Branch:** `feat/ux-realignment-m6-lapidacao` (30+ commits from `trunk`)
**Spec:** `docs/superpowers/specs/2026-06-22-harpia-m6-lapidacao-design.md`
**Plan:** `docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m6-lapidacao.md`

## Status: M6 implementation complete; awaiting human review + merge.

Built by a mix of subagents (clean-context), Cursor (Composer 2.5), and direct
work, all on one branch with disjoint file sets (no branch switching,
explicit-path commits). Integration was non-trivial — see "Integration fixes".

---

## Phase completion

| Phase | Scope | Status | By |
|---|---|---|---|
| 0 | Branch | ✓ | — |
| 1 | Foundation: motion lib, Skeleton, 5-layer tokens, focus-ring, tone doc | ✓ | Cursor + me |
| 2 | Backend: planassistant → BINDING_MATRIX + landing; NextTurn on promotion | ✓ | me |
| 3 | Plan thread: BindingMatrixCard, LandingCard, dispatcher; delete ConfirmCard | ✓ | subagent |
| 4 | Canvas: live state, node vocabulary, edge anchoring, hover card, vignette | ✓ | subagent |
| 5 | Sidebar count badges (Needs you / Your plans) | ✓ | Cursor |
| 6 | Mechanical sweep: motion + tokens + skeletons + gold discipline across pre-M6 components; `--token-danger` | ✓ | Cursor |
| 7 | IA cleanup: delete legacy routes + redirects; dev-login persona collapse | ✓ | Cursor + me |
| 8 | Zitadel branding: theme.css, assets, email message-texts, helm parity | ✓ | subagent |
| 9 | Verification (this log) | ✓ | me |

## Test + lint status

- **Frontend:** `npx vitest run` → **284/284 passed** (incl. theme-parity, matrix, motion, nav, settings).
- **Frontend lint:** `npm run lint` → clean (prettier + eslint).
- **Frontend types:** `npm run check` → **12 errors, all PRE-EXISTING on trunk** (0 M6 commits touch these files): `auth-roles.test.ts` (6), `artifact-flow.ts:83`, `ScheduleDialog.svelte:54`, `+layout.svelte` `resolve(string)` ×2, `plans/[templateId]/+page.svelte` ×2. Documented, out of M6 scope.
- **Backend:** `go test ./...` → 17 packages ok, **0 failures**; `go vet` clean.

## Bugs found + fixed during M6 (root-caused, not patched)

1. **Canvas blank from the expand button** (`4c08358`) — `PlanCanvasNode` called `onDestroy` in a context with a null lifecycle, crashing every node. Replaced with `$effect` teardown (Svelte 5 idiom). The 429s were the crash-induced remount/retry loop.
2. **Light/dark theming broken** (`fa10fcb`) — the 5-layer surface tokens were defined once with fixed dark values instead of per-mode. Split into `:not(.dark)`/`.dark` across all themes; added `theme-parity.test.ts` regression guard.
3. **Executor bindings not reaching the canvas** (`4b8d34f`) — `editBinding` only `.map`-updated existing bindings; first-time binds (the normal DRAFT case) were silently dropped. Now upserts. Server resolves SKU+kind from the installation id, so installation id alone is sufficient.

All three traced to the Phase-2 architectural shift (matrix card mutates config client-side; M5's server-side binding path was removed).

## Integration fixes (cross-boundary seams from disjoint-file parallelism)

- Admin surface was broken: `/admin/*` were redirect shims to the deleted top-level routes. Relocated the real pages into `/admin/*` (completing the spec's intended "move"), removed shims (`9677dc9`).
- Repointed dead links to deleted routes (`/oversee`→`/inbox`, `/tasks`→`/inbox`, `/integrations`→`/admin/integrations`).
- Aligned `nav.test.ts` + `settings.test.ts` to the post-M6 IA.

## Manual verification still recommended (could not run in sandbox)

The e2e sandbox rejects non-UUID config ids and has no seeded plan, so the
following were verified by code-path analysis + unit tests, not live render.
**Please confirm against real plan data:**
- Canvas renders bound/unbound nodes matching the matrix card (parity fix).
- Selecting an executor updates canvas + cost pill + enables Save.
- Canvas loads from the expand button (no blank, no 429 loop).
- Zitadel login shows Harpia branding after a cluster bootstrap.

## Known follow-ups (out of M6 scope, flagged not silently dropped)

1. **Stale e2e journeys** (`engineer-journey`, `leader-journey`, `home.smoke`) navigate deleted routes (`/agents`, `/integrations`, old `/tasks` home flow). They encode pre-M6 user flows and need a rewrite, not just route renames. Own task.
2. **RBAC primitive rename** — dev-login UI shows Ana/Platform Engineer but the underlying `HarpiaRole` primitives stay `Leader`/`Engineer`/`Overseer` (the whole app + backend depend on them). Renaming is a separate, risky refactor; deferred by decision.
3. **Zitadel real assets** — final Bodoni Moda wordmark + self-hosted WOFF2 fonts + optional background PNG (see `deploy/dev/kind/assets/NOTES.md`). Current logo is a serif-fallback placeholder; email message-texts ship as structured copy (Zitadel's API doesn't accept full HTML).
4. **Pre-existing svelte-check errors (12)** — predate M6; a separate cleanup.

## Next milestones (the real goal)

Per `docs/superpowers/specs/2026-06-22-aiuna-mvp-roadmap-revision.md`, with M6
closing the design realignment, the LinkedIn-plan real implementation is next:
- **M8 — real LLM providers** (BYOK + dynamic model discovery + real prices).
- **M7 — real integrations** (RSS pulls, LinkedIn publishes).
Each gets its own brainstorm → spec → plan cycle.
