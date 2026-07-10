## ADDED Requirements

### Requirement: Tenant isolation is enforced by Postgres RLS

The system SHALL enforce tenant isolation at the data layer with Postgres Row-Level Security,
independent of any external authorization service. A request SHALL only read or write rows for
the tenant resolved from its `identity.RequestContext`.

#### Scenario: Cross-tenant read is blocked

- **WHEN** a request authenticated for tenant A queries a resource owned by tenant B
- **THEN** RLS returns no rows for tenant B, regardless of application-layer checks

#### Scenario: Tenant is required at the boundary

- **WHEN** a tenant-scoped RPC is called without a resolvable tenant membership
- **THEN** the identity interceptor rejects the request before it reaches the handler

### Requirement: Coarse roles gate sensitive actions via app-level checks

The system SHALL authorize sensitive actions with a small set of coarse roles
(e.g. `admin`, `leader`, `overseer`, `owner`) checked in application code, without a
relationship-based authorization service.

#### Scenario: Admin-gated action requires the admin role

- **WHEN** a non-admin user attempts a tenant-admin action (e.g. mutating LLM provider config)
- **THEN** the handler denies it based on the caller's roles

#### Scenario: Approval/elicitation is limited to leader or assigned overseer

- **WHEN** a user who is neither a leader-role holder nor the assigned overseer attempts to
  answer an approval or elicitation
- **THEN** the handler denies it

### Requirement: No ReBAC authorization service in any deployment

The system SHALL NOT depend on OpenFGA (or any external ReBAC service) at runtime, in
configuration, or in any deployment manifest. Authorization is RLS + app-level role checks only.

#### Scenario: No OpenFGA in deployments

- **WHEN** the platform is deployed (kind, compose, or Helm)
- **THEN** no OpenFGA service, datastore database, bootstrap job, or authorization model is
  provisioned

#### Scenario: No OpenFGA configuration is read

- **WHEN** the control-plane starts
- **THEN** it reads no OpenFGA URL, store id, or authorization-model id, and no code path calls
  a ReBAC check

### Requirement: Downstream authorization is deferred to the system of record

Actions the platform performs on a downstream system of record (e.g. Odoo) MUST be authorized
by that system's own permission model, via the scoped credentials on its ExecutorInstallation.
The platform SHALL NOT mirror or re-implement the downstream system's authorization.

#### Scenario: Downstream action is bounded by installation credentials

- **WHEN** a plan step acts on a downstream system through its ExecutorInstallation
- **THEN** what the action may do is bounded by that installation's scoped credentials and the
  downstream system's own permission enforcement
