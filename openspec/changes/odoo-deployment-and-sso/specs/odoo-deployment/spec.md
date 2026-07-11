## ADDED Requirements

### Requirement: Optional, isolated Odoo Helm release

The system SHALL provide Odoo 19 Community Edition as a standalone Helm chart under `deploy/odoo/`. The chart SHALL deploy as a separate release in namespace `odoo` and SHALL not add Odoo as a dependency of `deploy/harpia/`.

#### Scenario: Odoo installs independently

- **WHEN** an operator installs the `deploy/odoo/` chart with its required Odoo Secrets and values
- **THEN** Helm renders and installs Odoo resources in the `odoo` namespace without modifying the Harpia release

#### Scenario: Harpia remains available without Odoo

- **WHEN** an operator deploys Harpia without installing the Odoo chart, or uninstalls the Odoo release
- **THEN** Harpia remains deployable and operational without an Odoo Pod, Odoo database, or Odoo configuration

### Requirement: Dedicated Odoo persistence

The Odoo release SHALL use its own PostgreSQL instance and database, separate from the PostgreSQL storage and credentials used by Harpia and Zitadel. The Odoo filestore SHALL be mounted from a persistent volume owned by the Odoo release.

#### Scenario: Odoo uses isolated PostgreSQL

- **WHEN** the Odoo Deployment starts
- **THEN** it connects only to the PostgreSQL Service and database configured by the Odoo chart's Odoo-specific Secret and values, and it does not use the Harpia/Zitadel PostgreSQL Service or Secret

#### Scenario: Filestore survives Odoo restart

- **WHEN** an Odoo Pod is restarted or rescheduled while its filestore PVC is retained
- **THEN** the replacement Pod mounts the same filestore volume and previously stored filestore data remains available

### Requirement: Odoo image includes pinned authentication addons

The Odoo image SHALL extend Odoo 19 Community Edition and SHALL include a pinned OCA `auth_oidc` addon plus the repository-owned `harpia_auth_zitadel` addon. The OCA dependency SHALL be pinned to an immutable, Odoo-19-compatible revision; production image builds SHALL NOT fetch a floating branch or unpinned dependency at runtime.

#### Scenario: Authentication modules are installable

- **WHEN** the Odoo image is built and initialized against its dedicated PostgreSQL database
- **THEN** Odoo can install both `auth_oidc` and `harpia_auth_zitadel` from its configured addon paths

#### Scenario: OCA revision is auditable

- **WHEN** an operator inspects the Odoo image build definition
- **THEN** the exact immutable OCA source revision used for `auth_oidc` is recorded alongside its repository provenance

### Requirement: Separate Zitadel OIDC client for Odoo

The Odoo release SHALL authenticate through Zitadel using an Odoo-specific OIDC client. The Odoo client ID, redirect URIs, sessions, and OIDC tokens SHALL be separate from Harpia's OIDC client, sessions, and tokens. An idempotent bootstrap mechanism SHALL create or reconcile the Odoo client without modifying Harpia's client configuration.

#### Scenario: Odoo client registration

- **WHEN** the Odoo OIDC bootstrap Job runs with its required Odoo-scoped Zitadel management credential and configured Odoo URLs
- **THEN** it creates or reconciles a separate Odoo OIDC client with only the configured Odoo redirect and logout URIs and publishes only the non-secret client metadata required by Odoo

#### Scenario: Harpia client remains untouched

- **WHEN** the Odoo OIDC bootstrap Job completes
- **THEN** the existing Harpia OIDC client ID and `harpia-oidc-config` remain unchanged

### Requirement: Pre-provisioned-user-only OIDC login

The `harpia_auth_zitadel` addon SHALL deny OIDC login unless the asserted identity resolves to a pre-existing Odoo user and the OIDC token contains `email_verified` set to `true`. The addon SHALL NOT create an Odoo user during login and SHALL NOT use an unverified email address as a fallback identity match.

#### Scenario: Pre-provisioned identity signs in

- **WHEN** Zitadel returns a valid Odoo-client OIDC response for an identity already linked to an Odoo user and includes `email_verified: true`
- **THEN** Odoo completes the OIDC login subject to its normal authorization checks

#### Scenario: Unknown subject is denied

- **WHEN** Zitadel returns a valid Odoo-client OIDC response whose identity has no linked Odoo user
- **THEN** Odoo denies the login and does not create a user, OAuth identity record, or other account linkage

#### Scenario: Unverified email is denied

- **WHEN** Zitadel returns an OIDC response with `email_verified` absent or false
- **THEN** Odoo denies the login even if an otherwise matching Odoo user exists

### Requirement: TLS-protected Odoo web ingress

The Odoo chart SHALL expose the Odoo web Service through a `networking.k8s.io/v1` Traefik Ingress with an explicitly configured hostname and TLS Secret. The Odoo and PostgreSQL Services SHALL remain ClusterIP-only.

#### Scenario: HTTPS ingress routes the Odoo UI

- **WHEN** the chart is installed with a valid ingress host and pre-provisioned TLS Secret
- **THEN** the Traefik Ingress has `ingressClassName: traefik`, a TLS entry for that host, and routes HTTPS traffic to the Odoo web Service

#### Scenario: TLS configuration is omitted

- **WHEN** production-oriented chart values omit the required ingress host or TLS Secret name
- **THEN** chart validation fails rather than rendering an unintended public HTTP ingress

### Requirement: Optional kind/Tilt development stack

The repository SHALL provide an explicit kind/Tilt path to build, deploy, bootstrap, and inspect the optional Odoo stack locally. That path SHALL preserve the existing Harpia-only Tilt workflow when Odoo resources are not enabled.

#### Scenario: Developer enables local Odoo

- **WHEN** a developer enables the documented Odoo Tilt resources and supplies the required local-only Secrets
- **THEN** Tilt builds the Odoo image, applies the Odoo namespace/resources and OIDC bootstrap Job in dependency order, and exposes an Odoo development endpoint for smoke testing

#### Scenario: Developer uses Harpia-only workflow

- **WHEN** a developer runs the existing Harpia development workflow without enabling Odoo
- **THEN** Tilt does not require an Odoo image, Odoo database, Odoo credentials, or Odoo OIDC client
