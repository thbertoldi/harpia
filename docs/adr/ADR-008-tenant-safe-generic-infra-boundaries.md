# ADR-008: Tenant-Safe Boundaries for Generic Infrastructure

**Status:** Accepted
**Date:** 2026-06-09
**Deciders:** thbertoldi

## Context

ADR-006 classifies Valkey and Garage as adopted generic infrastructure. They are not part of the core domain model, but they can still leak tenant data if domain code constructs raw cache keys or object paths directly.

Issue #29 requires tenant scoping at the boundary for both systems:

- Valkey keys must be prefixed with `t:{tenant_id}:`.
- Garage object paths must be tenant-scoped.
- Wrappers must derive tenant identity from `identity.RequestContext`.
- Callers must not bypass the wrappers.

## Decision

### Valkey

Raw Valkey access is confined to `control-plane/internal/cache/valkey.go`. Application code receives `cache.TenantStore`, which accepts logical keys such as `task:{task_id}` or `capability:{hash}` and derives the concrete key from the selected tenant in `identity.RequestContext`.

Concrete key format:

```text
t:{tenant_id}:{logical_key}
```

The wrapper rejects keys that already start with `t:`. This prevents callers from smuggling a pre-scoped key for a different tenant.

### Garage

Use one shared Garage bucket with a tenant prefix:

```text
tenant/{tenant_id}/{logical_path}
```

Rationale:

- The current dev and Helm manifests run one Garage instance and do not provision tenant buckets.
- A shared bucket keeps local development and bootstrap scripts simple.
- The boundary still gives object-level isolation because domain code never constructs the concrete path directly.
- Bucket-per-tenant can be revisited when tenant lifecycle automation includes provisioning and cleanup hooks.

The object-store wrapper rejects logical paths that already start with `tenant/`, and it can validate an existing object path against the current request tenant before reads or deletes.

## Migration Plan

The current development environments are expected to have no durable un-prefixed Valkey keys or Garage objects.

Before enabling this in an environment with persistent data:

1. Stop API writers.
2. Scan Valkey for keys that do not match `t:*`.
3. For each known cache family, rewrite the key using the owning tenant if it can be determined from the value or backing Postgres row.
4. Delete any unowned cache key; Valkey contents are rebuildable.
5. Scan Garage for objects outside `tenant/*`.
6. Move objects to `tenant/{tenant_id}/...` only when the owning tenant can be proven from Postgres metadata.
7. Quarantine or delete unowned objects rather than guessing ownership.
8. Restart API writers with the tenant-safe wrappers enabled.

## Enforcement

The Go test suite includes a source scan that fails if production code imports `go-redis` or reaches into the raw Valkey client outside `internal/cache/valkey.go`.

Unit tests cover:

- Valkey prefix derivation from `RequestContext`.
- rejection of already tenant-scoped Valkey keys.
- Garage object path derivation from `RequestContext`.
- rejection of cross-tenant Garage paths.

## Consequences

Domain handlers now pass logical cache keys and object paths only. Concrete Valkey and Garage naming remains an infrastructure concern.

Rate limiting is implemented as a Connect interceptor so it runs after identity context resolution and can use the same tenant-safe Valkey wrapper.
