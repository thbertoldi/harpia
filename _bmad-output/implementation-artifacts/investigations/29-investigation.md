# Investigation: Issue 29 Tenant-Safe Valkey and Garage Boundaries

## Hand-off Brief

Issue #29 is confirmed: adopted generic infrastructure did not have a complete tenant-safe boundary. Valkey exposed raw key operations to handlers and rate limiting, while Garage had manifests/configuration but no object-path adapter. The fix direction is to keep concrete Valkey/Garage naming inside infrastructure wrappers that derive tenant scope from `identity.RequestContext`.

## Case Info

- Date: 2026-06-09
- Input: GitHub issue #29, ADR-006, ADR-003, cache code, Garage manifests, identity request context
- Status: Concluded

## Problem Statement

Valkey keys and Garage object paths must be scoped per tenant at the infrastructure boundary so a bug in callers cannot read or write another tenant's adopted-infra data.

## Evidence Inventory

| Evidence | Grade | Notes |
|---|---|---|
| `docs/adr/ADR-006-domain-driven-design.md` classifies Valkey and Garage as generic/adopted infrastructure | Confirmed | Generic subdomains should not contaminate core domain logic. |
| `docs/adr/ADR-003-data-architecture.md` names Valkey for cache and Garage for S3-compatible object storage | Confirmed | Both are shared multi-tenant infrastructure. |
| `control-plane/internal/cache/valkey.go` exported raw `Get`, `Set`, and `Delete` methods | Confirmed | Callers could construct arbitrary keys. |
| `control-plane/internal/cache/ratelimit.go` reached into `client.rdb.Eval` directly | Confirmed | Raw Valkey bypass existed inside a secondary cache abstraction. |
| `control-plane/internal/tasks/handler.go` constructed concrete cache keys with tenant IDs | Confirmed | Domain handler knew cache key layout. |
| `control-plane/internal/cache/agent_cache.go` included tenant ID in its logical capability key | Confirmed | Cache family mixed logical and physical scoping. |
| Garage manifests/config exist, but no control-plane Garage adapter exists | Confirmed | No code boundary enforced tenant object paths. |
| `control-plane/internal/identity/context.go` exposes `RequireSelectedTenant` from `RequestContext` | Confirmed | Existing request context can be the boundary source of tenant identity. |

## Hypotheses

| ID | Hypothesis | Status | Resolution |
|---|---|---|---|
| H1 | Tenant-safe Valkey can be implemented without changing domain APIs by wrapping logical keys at the cache boundary. | Confirmed | `cache.TenantStore` prefixes keys from `RequestContext`. |
| H2 | Garage should use a shared bucket with `tenant/{tenant_id}/` prefixes for the current dev/prod shape. | Confirmed | ADR-008 documents the choice and migration plan. |
| H3 | Rate limiting must move behind identity to use `RequestContext`. | Confirmed | Implemented as a Connect interceptor after the identity interceptor. |

## Final Conclusion

Confidence: High. The issue is a boundary leak, not a missing domain rule: adopted infrastructure naming was partially owned by handlers and secondary cache types. The implementation should centralize concrete names inside Valkey and object-store wrappers, keep callers on logical names, and enforce the rule with unit tests plus a source scan in Go CI.
