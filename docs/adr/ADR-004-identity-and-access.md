# ADR-004: Identity and Access Management

**Status:** Accepted
**Date:** 2026-06-03
**Deciders:** thbertoldi

## Context

Harpia is multi-tenant. Each tenant has its own users, workspaces, agents, and tasks. We need:

1. **Authentication (AuthN):** Who are you? SSO with OIDC/SAML support.
2. **Authorization (AuthZ):** What are you allowed to do? Relationship-based access for nested resources (tenant → workspace → task → agent).
3. **Self-hosted and open-source.**

We originally considered Authentik. After reviewing alternatives, we chose Zitadel for authN and retained OpenFGA for authZ.

## Decision

### Authentication: Zitadel

**Zitadel** (Apache 2.0, Go) replaces Authentik for authentication. Rationale:

- **Lighter weight** than Authentik (single Go binary vs Python/Django).
- **Modern API-first design** — gRPC/Connect-native, fits our stack.
- **Multi-tenancy built-in** — Zitadel has organizations (our tenants) as a first-class concept.
- **Apache 2.0** — fully open-source, no AGPL concerns.

### Authorization: OpenFGA

**OpenFGA** (Apache 2.0) handles relationship-based access control (ReBAC). It implements Google's Zanzibar model:

```
define user
define tenant
  define member: user
  define workspace
    define editor: user or member
    define viewer: user or member
define task
  relation workspace: workspace
  permission view: viewer from workspace or editor from workspace
  permission edit: editor from workspace
```

OpenFGA walks the relationship graph to answer questions like "Can user U edit task T?" without application-level traversal logic.

### Separation of Concerns

| System | Responsibility | Example |
|---|---|---|
| Zitadel | Who you are | "I am alice@tenant-a.com" |
| OpenFGA | What you can do | "Alice can view tasks in workspace X" |
| PostgreSQL RLS | Data-level enforcement | `WHERE tenant_id = current_tenant_id()` |

## Rationale

### Alternatives Considered

| Alternative | Why not |
|---|---|
| Authentik | Heavier (Python/Django). AGPL-3.0 concerns. Zitadel is leaner and Go-native. |
| Keycloak | Heavy JVM stack. Complex configuration. Not lean. |
| Clerk / Auth0 | Not self-hosted. Violates our philosophy. |
| OPA (Rego) | General-purpose policy engine. Relationship traversal must be hand-coded. |
| Casbin | Library, not a service. Requires embedding in every service. |
| SpiceDB | Functionally equivalent to OpenFGA. OpenFGA has stronger CNCF momentum and Okta backing. |
| Keto (Ory) | Also Zanzibar-based, but smaller ecosystem and less CNCF traction. |

## Consequences

- **Easier:** AuthN and AuthZ are cleanly separated. Zitadel handles SSO protocols (OIDC, SAML) so we don't have to. OpenFGA's ReBAC model maps naturally to our nested resource structure.
- **Harder:** Two additional services to operate. The authZ model (Zanzibar tuples) requires upfront design of the relationship graph.
- **Next:** Write the OpenFGA authorization model. Configure Zitadel as the OIDC provider in SvelteKit.
