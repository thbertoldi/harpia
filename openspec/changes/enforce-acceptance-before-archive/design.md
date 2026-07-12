## Context

The current workflow proves build and unit-test health but permits a change to be
archived after an unchecked or deferred manual smoke task. It also publishes container
images without exercising a tenant-safe PostgreSQL boundary or the embedded catalog
through production configuration logic. This change applies the Constitution's
tenant-safe boundary and PlanConfiguration readiness invariants to delivery itself.

## Goals / Non-Goals

**Goals:**

- Make archived OpenSpec changes mechanically reject incomplete, deferred, and
  unsupported N/A work, and require final acceptance evidence.
- Run a fail-loud, deterministic acceptance package against the real embedded
  `linkedin-content-studio` PlanTemplate, validators, audit Repository, and image
  provider resolver.
- Prevent container publication until migration-backed acceptance and archive hygiene pass.
- Remove confirmation overrides from the tracked archive adapters.

**Non-Goals:**

- Replace OpenSpec's CLI archive implementation or add a general workflow engine.
- Run full Temporal workflows or external LinkedIn/image-provider calls in CI.
- Modify product-domain behavior, templates, audit implementation, or frontend surfaces.

## Decisions

### Delivery check scans archived artifacts, not active changes

`scripts/ci/openspec-delivery-check.sh` will iterate `openspec/changes/archive/*/tasks.md`
and associated completion notes. It rejects unchecked boxes, `agent-incapable`, unresolved
`DEFERRED`, missing final `mise run acceptance` task, and unchecked conditional tasks without
an explicit `N/A because no <surface> files changed` reason. Existing archive drift remains
visible rather than being auto-repaired.

Scanning archives makes the invariant durable after a directory is moved, while the OpenSpec
tasks rule and archive adapters prevent new violations before that point. A shell script is
chosen over parsing OpenSpec internals because archives are Markdown artifacts and the gate
must run in CI and locally with no provider dependency.

### Acceptance uses the production Go adapters and PostgreSQL app role

`control-plane/internal/acceptance` will be a separate integration package invoked only by
`mise run acceptance`. Its shared setup fails with `t.Fatalf` if
`HARPIA_TEST_DATABASE_URL` is absent or connects as a superuser. It documents the matching
app-role SQL beside the test setup. It seeds actual artifact types, ExecutorSKUs, and the
embedded template, then uses repository/adapter APIs instead of synthetic template copies.

The suite tests the deterministic boundaries: optional PlanStep participation and slot-binding
readiness (A1), forced-RLS audit persistence/dedupe/redaction (A2), and the noop image
configuration/resolver (A3). LinkedIn uses `approval_only`; image uses `noop`, avoiding live
credentials. Full Temporal execution is an explicit TODO skip because it requires worker
wiring outside this change.

### CI provisions PostgreSQL and a NOBYPASSRLS app role

The acceptance job uses pgvector Postgres 16, runs Atlas migrations under the service
superuser, creates/grants the non-superuser app role, and passes its DSN only to acceptance.
The job runs both `mise run acceptance` and `mise run openspec-delivery-check`; published
containers depend on it. This mirrors the audit integration test's RLS requirement rather
than allowing a privileged connection to make tenant assertions tautological.

### Archive adapters hard-stop before moving a change

The skill and command retain artifact and task checks but replace their confirmation paths
with a stop. They also require successful `mise run acceptance` evidence according to the
OpenSpec task policy before an archive operation can proceed. Delta-spec synchronization keeps
its existing explicit choice because it is separate from delivery completion.

## Risks / Trade-offs

- [Existing archives fail the new delivery check] → report the archive paths as pre-existing
  drift; do not alter unrelated historical changes in this change.
- [Acceptance database setup is unavailable locally] → the suite fails loudly with setup SQL;
  CI provisions the role deterministically.
- [Catalog seed dependencies change] → acceptance uses the same public seed functions and
  fails if the embedded catalog no longer loads through production paths.
- [Markdown evidence is forged or malformed] → the delivery check validates the required
  machine command and task status; stronger attestations are out of scope.

## Migration Plan

1. Land policy, archive adapter, mise, acceptance package, and CI wiring together.
2. CI immediately blocks publication if acceptance or archive hygiene fails.
3. Triage any historical archives reported by the new check in dedicated follow-up changes;
   this change does not rewrite them.
4. Roll back by reverting this single change; no schema migration is introduced here.

## Open Questions

None. The acceptance boundary, external adapter modes, archive behavior, and CI role model are
settled by this change.
