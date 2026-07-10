## Why

OpenFGA (ReBAC) is provisioned as infrastructure but **never called by application code** —
zero tuple writes, zero permission checks anywhere in the control-plane or frontend. The real
authorization today is Postgres RLS (tenant isolation) + the identity interceptor
(`RequireTenant`) + thin app-level role-string checks (`admin`/`leader`/`overseer`/`owner`,
sourced from `users.role`). So OpenFGA is pure carrying cost: a running service, its own
Postgres database, a bootstrap Job, an authorization model, port-forwards, env-sync scripts,
and Tilt wiring — all for permissions the product does not yet need. The
[constitution §11](../../../docs/architecture/harpia-platform.md#11-áreas--rbac) has been
updated to retire it: RLS + Zitadel/coarse roles + thin checks now, ReBAC deferred to a future
enterprise tier.

## What Changes

- **Remove all OpenFGA infrastructure** (Helm template, kind manifests, compose service,
  bootstrap Job + ReBAC model, its Postgres `openfga` database, Tilt wiring, `mise` task, and
  the `sync-fga-config.sh` / `port-forward-openfga.sh` scripts). **BREAKING** (pre-v1): the
  OpenFGA deployment and `openfga` DB are removed.
- **Delete dead OpenFGA config** in the control-plane (`OpenFGAURL`, `OpenFGAStoreID`,
  `OpenFGAAuthorizationModelID` fields + their env loads) and the aspirational
  "swap for an FGA check" comment.
- **Strip `OPENFGA_*` / `PUBLIC_OPENFGA_*`** keys from the generated `.env.local` files.
- **Keep unchanged:** Postgres RLS (the real tenant-isolation boundary), the identity
  interceptor / `RequireTenant`, and the app-level role checks. **No runtime authz path
  changes** — behavior before and after is identical.
- Pin the surviving authorization model in a spec so the end-state is testable and documented.

## Capabilities

### New Capabilities
- `authorization-model`: the platform's authorization model after retiring ReBAC — tenant
  isolation via Postgres RLS, coarse roles via app-level checks, downstream authz deferred to
  the system of record, and no OpenFGA service in any deployment.

### Modified Capabilities
<!-- none — no existing capability spec covers authz, and no runtime behavior changes -->

## Impact

- **control-plane:** `internal/config/config.go` (delete 3 fields + env loads),
  `internal/llm_config/handler.go` (delete stale comment). No authz logic changes.
- **deploy:** delete `deploy/harpia/templates/openfga.yaml`, `deploy/dev/kind/openfga.yaml`,
  `deploy/dev/kind/openfga-bootstrap.yaml`, `deploy/dev/kind/openfga-model.fga`; remove the
  `openfga:` blocks from `values.yaml`/`values-dev.yaml`/`values-prod.yaml` and the
  `openfga` service from `deploy/dev/compose.yaml`; drop the `CREATE DATABASE openfga` +
  GRANT from `deploy/dev/kind/postgres.yaml` and `deploy/harpia/templates/postgres.yaml`.
- **Tiltfile / mise / scripts:** remove all `openfga`/`fga` resources, deps, the
  `dev-openfga-migrate` task, and `scripts/{sync-fga-config,port-forward-openfga}.sh`.
- **config/env:** strip `OPENFGA_*` (control-plane) and `PUBLIC_OPENFGA_*` (frontend) keys.
- **proto:** no message/RPC changes; optionally reword the `llm_config.proto` "tenant.admin"
  doc comment (it's a role check, not FGA).
- **docs:** scrub OpenFGA from `README.md`, `deploy/*/README.md`, `docs/ISSUES.md`,
  `docs/notes/2026-06-16-tenant-llm-config-design.md`. (Constitution §4/§6/§11 + ADR map and
  the ADR index already reflect the retirement.)
- **Do NOT touch:** `internal/database/postgres.go` `WithTenant`, the `000003`/`000010` RLS
  migrations, or the `harpia`/`zitadel` Postgres databases.
- **Verification:** `go test ./...` (RLS + identity + approvals/elicitations tests still green),
  `mise run helm-lint`, `mise run dev` comes up with no OpenFGA resource, `bun run check`.
