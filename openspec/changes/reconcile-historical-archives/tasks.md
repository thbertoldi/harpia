# Tasks: reconcile-historical-archives

## 1. Reconcile each grandfathered archive

For each archive under `openspec/changes/archive/` dated before 2026-07-12
that `openspec-delivery-check` flags (currently: the 2026-07-01, 2026-07-03,
2026-07-07, 2026-07-09, and 2026-07-10 families):

- [ ] 1.1 Resolve unchecked tasks: check the ones completed, or move genuinely-incomplete work to a linked follow-up change.
- [ ] 1.2 Remove or replace every `agent-incapable` note with acceptance evidence or a `Human verification:` record.
- [ ] 1.3 Resolve every `DEFERRED` entry by linking a follow-up change or marking done.
- [ ] 1.4 Ensure each `tasks.md` ends with a checked acceptance task (exact `mise run acceptance`) or a structured `Human verification:` record.

## 2. Verify and retire the grandfather clause

- [ ] 2.1 Run `bash scripts/ci/openspec-delivery-check.sh` and confirm it passes with NO grandfathered archives flagged.
- [ ] 2.2 Once all historical archives are clean, tighten `scripts/ci/openspec-delivery-check.sh` by removing the date-cutoff grandfather clause (enforce on all archives unconditionally). **Acceptance:** Given the clause is removed, when the check runs, then it still passes because every archive is now clean. **Focused verification:** `bash scripts/ci/openspec-delivery-check.sh`.
