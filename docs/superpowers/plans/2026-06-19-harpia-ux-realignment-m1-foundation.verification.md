# M1 Verification — 2026-06-20

Branch: `feat/ux-realignment-m1-foundation`
Plan: `docs/superpowers/plans/2026-06-19-harpia-ux-realignment-m1-foundation.md`

## Automated checks (executed)

| Check | Result | Notes |
|---|---|---|
| `npm run test` | ✅ 222/222 passing across 36 files | Includes 7 new tests in `sections-m1.test.ts`, 9 in `storage.test.ts`, 3 in `flags.test.ts`, 1 in `hardcoded-copy.test.ts` (i18n parity) |
| `npm run check` (svelte-check + tsc) | ✅ 10 errors, 0 new | All 10 errors pre-existing on trunk (auth-roles.test.ts ×6, artifact-flow.ts ×1, +layout.svelte ×1 typed-route, plans/[id]/+page.svelte ×2). M1 changes introduced zero new type errors. |
| `npm run lint` (prettier + eslint) | ✅ clean | Required a chore-commit (`f988fb7`) to apply prettier formatting to the 5 new files — implementer subagents committed before formatting. |
| Dev server boot (flag ON) | ✅ Vite ready in 451 ms | `PUBLIC_FEATURE_UX_REALIGNMENT_M1=true npm run dev` — no compile errors, no runtime warnings on startup |
| Route resolution (unauthenticated curl, flag ON) | ✅ all return 302 → /login | `/`, `/inbox`, `/discover`, `/new`, `/plans/test-id/canvas`, `/admin/integrations`, `/admin/agents` all hit the auth redirect (which confirms the routes exist in the SvelteKit table and the layout loader runs). |

## Reviewer-verified behavior (per Task 7 review)

The Task 7 reviewer (sonnet) executed named-risk checks against the layout diff and confirmed:

- **Flag-off byte-identity:** with `m1Enabled === false`, the `sections` derived passes identical arguments to legacy `resolveNavSections` — exact equivalence to pre-change behavior.
- **SSR localStorage guard:** the `$effect` block guards with `typeof localStorage !== "undefined"`.
- **Persona persistence:** `togglePersona()` updates reactive state AND writes to localStorage (also SSR-guarded).
- **Toggle visibility gating:** `{#if canSwitchPersona}` combines `m1Enabled && canSwitchToAdminPersona(role)` — hidden when flag off OR role disallows.
- **Imports added cleanly, no duplicates, existing import order preserved.**
- **FeedbackBadge conditional untouched** (line 169 of layout, per plan §Task 7 Step 4).

## Browser-driven verification (manual — requires the user)

These cannot be executed from this CLI environment. Steps for the user to run:

```bash
cd frontend
PUBLIC_FEATURE_UX_REALIGNMENT_M1=true npm run dev
```

Then in a browser at `http://localhost:5173`:

| Step | Expected |
|---|---|
| Log in via `devLogin('Leader')` (or browser console / dev helper) | Sidebar shows **3 items**: Needs you, Discover, New plan. Header shows persona toggle reading "OPERATOR ⇄". |
| Click each operator sidebar item | Each navigates to the MilestoneStub page showing "Coming in M{2|4|5}". |
| Click the persona toggle | Switches to "PLATFORM ENGINEER ⇄"; sidebar now shows **4 items**: Integrations, Agents, Audit log, Tenant settings. |
| Click each admin sidebar item | Each redirects to the corresponding legacy route (`/integrations`, `/agents`, `/audit`, `/settings`); legacy pages render unchanged. |
| Reload the page | Persona stays on admin (localStorage persisted). |
| Log out, log in as `Overseer` | Toggle to admin: sidebar shows only **Audit log** (Overseer has only `viewAudit`). |
| Restart dev server **without** the env var (`unset PUBLIC_FEATURE_UX_REALIGNMENT_M1; npm run dev`) | Sidebar reverts to today's 10 items; persona toggle is gone; nothing else changes. |

If a `member` / unprivileged dev persona is reachable, also confirm: **flag on, member role → toggle hidden** (covered by unit test `canSwitchToAdminPersona("member") === false`, but worth a visual sanity check).

## Commits

```
1908cde  feat(ux-m1): add feature-flag helper
139b7dd  feat(ux-m1): add persona-mode storage helpers
a52e141  feat(ux-m1): add M1 navigation module
88f72f2  feat(ux-m1): add nav i18n keys for new IA
d1bc033  feat(ux-m1): scaffold operator-side route stubs
ed4f82d  feat(ux-m1): scaffold admin-side route redirects
ae1a4fa  feat(ux-m1): integrate persona-aware sidebar behind flag
f988fb7  chore(ux-m1): apply prettier formatting to new files
```

## Status

**Automated portion: ✅ complete.** Manual browser verification still required before merge.
