# Proposal: reconcile-historical-archives

## Summary

Reconcile the OpenSpec archives created before the acceptance gate
(2026-07-12) so the repository's historical delivery record meets the new
`openspec-delivery-check` standard. These archives are currently grandfathered
by `scripts/ci/openspec-delivery-check.sh`; this change removes the
grandfathering dependency by bringing each archive to a clean state.

## Motivation

The `enforce-acceptance-before-archive` change introduced a mandatory
delivery gate. Historical archives predate it and were grandfathered to keep
CI green. They carry real drift: unchecked tasks, `agent-incapable` notes,
unresolved `DEFERRED` entries, and missing acceptance evidence. This change
reconciles them so the grandfather clause can eventually be retired.

## Scope (per archive)

For each grandfathered archive under `openspec/changes/archive/`:

- Check or remove unchecked tasks; if a task was genuinely not done, move it
  to a linked follow-up change rather than leaving it unchecked.
- Replace any `agent-incapable` note with either acceptance evidence
  (`mise run acceptance` scenario or `Human verification:` record) or a
  follow-up reference.
- Resolve `DEFERRED` entries by linking a follow-up change or marking done.
- Ensure each `tasks.md` ends with a checked acceptance task or a structured
  `Human verification: YYYY-MM-DD | irreducibly external: <check> | verified by: <person>` record.

## Non-goals

- Re-opening delivered behavior. This is delivery-record hygiene only; no
  product behavior changes.
- Reverting the grandfather clause before reconciliation is complete.

## References

- Gate change: `openspec/changes/enforce-acceptance-before-archive/`
- Gate script: `scripts/ci/openspec-delivery-check.sh`
