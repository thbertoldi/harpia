## Why

Harpia has published features as complete while their user-facing behavior remained
unverified: acceptance tasks were deferred and archive adapters permitted unchecked
tasks after confirmation, while CI did not exercise production logic against a real
database. Delivery must make verified acceptance a release and archive prerequisite.

## What Changes

- Add a delivery-check gate that rejects archived changes with incomplete, deferred, or
  unsupported N/A tasks and requires verified acceptance evidence.
- Add a deterministic Go acceptance suite against production PlanConfiguration,
  audit, and image-executor logic using a non-superuser PostgreSQL app role.
- Make CI run migrations, acceptance, and the delivery check before container publication.
- Require non-deferrable machine-executable acceptance tasks in OpenSpec and make archive
  adapters hard-stop when completion or acceptance evidence is missing.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `openspec-workflow`: archive eligibility and behavior-change task requirements now
  require complete, verified acceptance without a confirmation override.

## Impact

- Adds `scripts/ci/openspec-delivery-check.sh`, `mise run acceptance`, and
  `mise run openspec-delivery-check`.
- Adds `control-plane/internal/acceptance/` integration tests and a PostgreSQL-backed CI job.
- Updates OpenSpec workflow policy and archive-adapter instructions, plus the repository
  delivery gate in `AGENTS.md`.
