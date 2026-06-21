# M2 Verification — 2026-06-20

Branch: `feat/ux-realignment-m2-unified-inbox`
Plan: `docs/superpowers/plans/2026-06-20-harpia-ux-realignment-m2-unified-inbox.md`

## Automated checks (executed)

| Check | Result | Notes |
|---|---|---|
| `npm run test` | ✅ 231/231 passing across 39 files | Includes 3 new tests in `feedback/counts.test.ts`, 3 in `inbox/buckets.test.ts`, 3 in `inbox/aggregator.test.ts`, plus the existing M1 + i18n parity tests |
| `npm run check` (svelte-check + tsc) | ✅ 10 errors, 0 new | Identical to trunk baseline (auth-roles.test.ts ×4, artifact-flow.ts ×1, +layout.svelte ×1 typed-route, plans/[id]/+page.svelte ×2, hardcoded-copy.test.ts ×2). M2 introduced zero new type errors. |
| `npm run lint` (prettier + eslint) | ✅ clean | All formatting + lint rules satisfied. |
| Dev server boot | ✅ Vite ready, no compile errors | `npm run dev` on port 5179 |
| Route resolution | ✅ all expected routes resolve (302 to auth) | `/inbox`, `/oversee` → /inbox, `/elicitations` → /inbox, `/approvals` → /inbox, both detail-page deep links |

## Dispatch + review log

| Task | Implementer | Reviewer | Commits | Notes |
|---|---|---|---|---|
| T1: feedback counts + InboxItem types | Codex | Claude haiku | `4230473` + `15ff462` (fix) | Brief bug (RPC pagination model) caught by implementer, fix dispatched |
| T2: inbox aggregator + buckets | Codex | Claude haiku | `2d6a3cf` | 3 brief revisions before landing (test/impl contradiction, function signature, TS null-narrowing) |
| T3: InboxBadge + sidebar swap | Codex (in worktree) | Claude haiku | `6ff7468` (cherry-picked from `fa02333`) | Parallel with T4 in `.claude/worktrees/m2-t3` |
| T4: inbox page + i18n + InboxRow | Cursor (in worktree) | Claude haiku | `e2fdd86` (cherry-picked from `e8d25e9`) | Parallel with T3; brief pre-flighted (formatRelativeTime shim, ArtifactPreview reuse) |
| T5: approval entry | Codex | Claude haiku | `13aa70a` | Brief v2 fix (footer snippet must be conditionally passed, not unconditionally with internal guard) |
| T6: redirect legacy oversight routes | Cursor (staged), orchestrator (committed) | Claude haiku | `2c496e7` | Cursor staged but didn't commit; orchestrator finalized after recovery from a bundled-commit accident |
| T7: detail-page back-links | Cursor | Claude haiku | `c5510b2` | Clean full cycle |

Also: `01a88aa` (orphan legacy back-link i18n keys cleanup), `16d2aeb` (.gitignore worktrees).

## Browser-driven verification (manual — required before merge)

Run `cd frontend && PUBLIC_FEATURE_UX_REALIGNMENT_M1=true npm run dev` and walk:

| Step | Expected |
|---|---|
| Visit `/oversee`, `/elicitations`, `/approvals` directly | Each 302-redirects to `/inbox` |
| Sidebar (flag off and flag on) | Shows a single "Needs you" entry with a badge equal to total pending. The three legacy oversight items are gone. |
| Visit `/inbox` (logged in, with pending items) | Header "Needs you · N pending", filter chips (All / Elicitations / Approvals / Feedback), row list rendering subtype pill + source breadcrumb + age + actions |
| Click filter chip | List filters to that subtype; counts unchanged |
| Approval row: click "Preview" | Row footer expands inline to show the input artifact via `ArtifactPreview`; button label changes to "Hide" |
| Approval row: click "Approve" | RPC fires, row marks as "Approved" (greyed), no further action available |
| Approval row: click "Reject" without text | Footer expands with a textarea labeled "Reason (required to reject)" — RPC not yet called |
| Approval row: click "Reject" with reason text | RPC fires, row marks as "Rejected" |
| Elicitation/Feedback row: click "Open thread" | Navigates to the existing detail page (graceful degrade — M3 will repoint) |
| Detail-page back-link | Returns to `/inbox` (not the deleted list route) |

## Commits

```
13aa70a feat(ux-m2): inline approval preview and approve/reject in the inbox
16d2aeb chore: gitignore .claude/worktrees/
c7ee22d docs(ux-m2): fix Task 5 brief; conditionally pass footer snippet to InboxRow
e2fdd86 feat(ux-m2): render the inbox page with filters and graceful-degrade open-thread
6ff7468 feat(ux-m2): replace three legacy badges with unified InboxBadge
bb9ff31 docs(ux-m2): drop stale InboxApprovalPreview reference from M2 file map
5a11ff4 docs(ux-m2): pre-flight T3+T4+T5 briefs (poll cadence, formatRelativeTime shim, reuse ArtifactPreview)
2d6a3cf feat(ux-m2): add inbox aggregator and earlier-today bucketing
9e0e66d docs(ux-m2): fix Task 2 brief; init resolveNext to noop instead of null
01a88aa chore(ux-m2): remove orphaned legacy back-link i18n keys
65b4059 docs(ux-m2): fix Task 2 brief; narrow resolveNext type and pin 'last' to InboxItem[]
2c496e7 feat(ux-m2): redirect legacy oversight routes to /inbox; collapse nav
c5510b2 feat(ux-m2): point elicitation+approval detail back-links to /inbox
c24bea0 docs(ux-m2): fix Task 2 brief; second test must loop past initial empty yield
6a14b6e docs(ux-m2): fix Task 2 brief; selectEarlierTodayItems returns InboxItem[]
39fff37 docs(ux-m2): fix Task 1 brief to use server-streaming RPC (AsyncIterable)
15ff462 fix(ux-m2): use AsyncIterable for listPendingFeedback (server-streaming RPC)
4230473 feat(ux-m2): add InboxItem types and feedback count helper
c68d3ce docs(ux-m2): write implementation plan for M2 unified inbox
```

## Whole-branch final review (Claude opus)

Verdict: **Ready to merge with fixes.** Two Critical findings, four Important, six Minor. Critical findings addressed below; Importants tracked as M3 follow-ups.

### Post-review fixes applied to the branch

| Finding | Fix commit | Notes |
|---|---|---|
| InboxBadge runtime crash (`Cannot read properties of null (reading 'r')` in `onDestroy`) — missed by both T3 and T9 reviewers | `3263178` | Removed redundant `onDestroy`; `$effect` cleanup handles unmount. Moved `timer` inside the effect so each run owns its instance. Plan patched in `ab0586d`. |
| Critical #1 — Feedback rows looped back to inbox (no submission path) | `23e21257` | Restored `frontend/src/routes/oversee/+page.svelte` from before its T6 deletion. Removed the redirect loader. `/oversee` is no longer in the sidebar but reachable via `InboxFeedbackActions`'s deep-link — M2 graceful-degrade until M3 ships the per-plan chat thread. |
| Critical #2 — Hardcoded English "Approve to publish" summary in aggregator | `8e2300a` | Aggregator now emits empty `summary` for approvals; `InboxRow` derives the displayed text via `translate("inbox.summary.approval", $locale)`. Added the key to both locale files. |

### Important findings deferred to M3 follow-ups (not merge blockers)

- **`formatRelativeTime` locale type laxity** (`lib/i18n/format.ts`) — signature is `(iso, locale: string)` instead of `(iso, locale: Locale)`; uses an unsafe `as Locale` cast. Works at runtime; cleanup pass.
- **`watchInbox` upstream-stream leak** (`lib/inbox/aggregator.ts`) — no `AbortSignal` plumbed through to the two underlying server-streaming RPCs. Pre-existing pattern in the codebase; M2 doubled it because the inbox subscribes to two streams. Worth a foundational fix in M3 when more streams stack.
- **Ad-hoc `.replace("{count}", …)` in translate calls** (`inbox/+page.svelte`, `InboxRow.svelte`) — codebase already supports `translate(key, locale, { count: N })`. Convert in a follow-up.
- **Note:** the "orphaned `oversee.*` i18n keys" Important from the review is mooted by Critical #1's fix — those keys are now live again because `/oversee/+page.svelte` was restored.

## Status

**Ready to merge.** Manual browser smoke from the checklist above is still required before merge — these post-review fixes have not been clicked-through in a live browser.

## Closes

- #204 — `demo: unify elicitations and approvals into pending actions inbox` (will be referenced in PR description)
