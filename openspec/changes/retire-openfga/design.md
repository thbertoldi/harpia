## Context

The constitution retired the OpenFGA/ReBAC authorization layer
([§11](../../../docs/architecture/harpia-platform.md#11-áreas--rbac)): it is too much for the
current stage — complexity for permissions the product does not yet need. A footprint audit
confirmed the decisive fact that makes this change trivial and safe:

> **OpenFGA is fully provisioned as infrastructure but never called by application code.**
> Zero FGA tuple writes, zero FGA checks, no FGA SDK dependency. The three `OpenFGA*` config
> fields are loaded but never read. The only Go mention besides config is an aspirational
> comment.

The actual authorization in force today, and after this change (unchanged):
- **Tenant isolation** — Postgres RLS (`WithTenant` sets `harpia.tenant_id`; policies in
  migrations `000003`/`000010`) + the identity interceptor `RequireTenant`.
- **Coarse roles** — app-level `hasRole`/`isLeaderRole` checks; roles come from `users.role`.

So this is a **deletion**, not a migration: remove a service (+ its `openfga` Postgres DB,
bootstrap Job, ReBAC model, port-forwards, env-sync, Tilt wiring) that does nothing.

## Goals / Non-Goals

**Goals:**
- Delete every OpenFGA touchpoint: deploy manifests, its Postgres DB, dev tooling, dead config,
  and generated env keys.
- Leave the real authorization (RLS + interceptor + role checks) exactly as-is.
- Pin the surviving authorization model in a spec so it's testable and documented.

**Non-Goals:**
- Changing any authorization *behavior* (there is none to change).
- Moving role source from Postgres `users.role` to Zitadel grants — a separate, optional
  follow-up, not required to remove OpenFGA.
- Building área-scoped or per-resource authorization (deferred; ReBAC returns at the
  enterprise tier per constitution §11).
- Touching RLS, `WithTenant`, or the `harpia`/`zitadel` databases.

## Decisions

- **Delete, don't shim.** Because no code path calls FGA, the whole infra + dead config can go
  in one change with no behavior-preserving intermediate. The checks that exist
  (`RequireTenant`, `hasRole`, `isLeaderRole`) *are* the end-state.
- **Keep RLS as the security boundary.** RLS is independent of FGA and is the real tenant
  guarantee; it is explicitly out of scope for deletion.
- **Keep the role model as-is** (`users.role` → `rc.Roles` → app checks). Zitadel-grant sourcing
  is a deferred nicety, not part of retirement.
- **Remove only the `openfga` database**, not the shared Postgres instance or the
  `harpia`/`zitadel` DBs (they co-reside).

## Risks / Trade-offs

- **Tilt dependency ordering** → several Tilt resources depend on `fga-sync`/
  `openfga-port-forward` for env generation; remove those deps in the same change or the
  env-sync step waits on a deleted resource. Mitigation: do Tiltfile edits and script deletion
  together, then `mise run dev` to confirm clean startup.
- **Stale generated env keys** → `OPENFGA_*` / `PUBLIC_OPENFGA_*` won't auto-delete after
  `sync-fga-config.sh` is removed. Mitigation: strip them manually in the same change so nothing
  reads a stale store/model id.
- **Accidental RLS damage** → the one real hazard. Mitigation: the change must not touch
  `internal/database/postgres.go` or the RLS migrations; `postgres_test.go` and
  `middleware_test.go` must stay green.
- **Loss of a future ReBAC head start** → accepted; the constitution documents the enterprise
  tripwire for reintroducing it, modeled against real patterns rather than the current unused
  aspirational model.

## Migration Plan

Pre-v1, no data migration for app data. Operationally, the `openfga` Postgres database can be
dropped after the manifests are removed. Suggested order:
1. Delete deploy manifests + values blocks + compose service + the `CREATE DATABASE openfga`
   lines.
2. Remove Tilt wiring, the `mise` task, and the two scripts together.
3. Delete the dead control-plane config fields + comment.
4. Strip `OPENFGA_*` / `PUBLIC_OPENFGA_*` env keys.
5. `go test ./...`, `mise run helm-lint`, `mise run dev` (no OpenFGA resource), `bun run check`.
6. Docs scrub.

## Open Questions

- Whether to drop the `openfga` Postgres database immediately in dev or leave it orphaned until
  the next environment reset (cosmetic; it's simply unused once manifests are gone).
