# openspec-workflow

## ADDED Requirements

### Requirement: Historical archive grandfathering

The delivery check SHALL grandfather OpenSpec archives created before the acceptance gate (2026-07-12) so the gate enforces going forward without retroactively blocking trunk on pre-gate delivery drift. Grandfathered archives SHALL be logged (not silently ignored) and reconciled by the `reconcile-historical-archives` change, after which the grandfather clause SHALL be retired.

#### Scenario: Pre-gate archive does not block the delivery check

- WHEN the delivery check scans an archive dated before 2026-07-12
- THEN it logs the archive as grandfathered and continues without failing
- AND it does not silently skip it

#### Scenario: Grandfather clause is retired once history is clean

- WHEN every grandfathered archive has been reconciled (unchecked tasks resolved, no `agent-incapable`, no unresolved `DEFERRED`, acceptance evidence present)
- THEN the date-cutoff grandfather clause is removed so the delivery check enforces on all archives unconditionally
