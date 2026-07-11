## Why

Aiuna operates on business data owned by downstream systems of record; Odoo is the first such system. The platform needs a self-hosted Odoo 19 Community deployment that users can enter through the same Zitadel identity provider while keeping Odoo operationally and security-wise separate from Harpia. This first roadmap slice establishes only the deployable downstream foundation and single sign-on. It deliberately does not connect Harpia to Odoo or create users in Odoo.

## What Changes

- Add a standalone `deploy/odoo/` Helm chart for Odoo 19 Community Edition, installed as a separate `odoo` release in the `odoo` namespace.
- Build an Odoo image that pins and installs OCA `auth_oidc` plus the repository-owned `harpia_auth_zitadel` hardening addon.
- Deploy Odoo with a dedicated PostgreSQL instance/database and a persistent filestore volume; neither storage service is shared with Harpia or Zitadel.
- Configure a separate Zitadel OIDC client for Odoo through an idempotent bootstrap Job. The client, relying-party session, and tokens remain distinct from Harpia's client and sessions.
- Add `harpia_auth_zitadel` to require an already provisioned Odoo user and a verified email claim before an OIDC login is accepted; it must deny just-in-time user creation and unknown subjects.
- Expose the Odoo web UI through a TLS-enabled Traefik Ingress, with hostname and pre-provisioned TLS Secret supplied by chart values.
- Add kind/Tilt local-development resources and documentation for the optional Odoo stack without making the existing Harpia stack depend on it.

## Capabilities

### New Capabilities

- `odoo-deployment`: A separately deployable, optional Odoo 19 Community downstream system with isolated PostgreSQL and filestore storage, Zitadel OIDC authentication, no JIT user provisioning, and TLS ingress.

### Modified Capabilities

(none)

## Impact

- **Deployment**: new `deploy/odoo/` Helm chart, Odoo image/addon source, and kind/Tilt manifests. Existing `deploy/harpia/` remains independent and does not acquire an Odoo dependency.
- **Identity**: a new, separate Odoo OIDC client is registered in the existing Zitadel instance. It does not alter Harpia's OIDC client, client ID, tokens, or sessions.
- **Storage**: Odoo owns a separate PostgreSQL Deployment/PVC and filestore PVC in the `odoo` namespace.
- **Control-plane, agent-runtime, frontend, proto, database**: no changes. The Odoo connector, user provisioning, downstream mutations, and custom Odoo business modules are expressly deferred.
- **Operations**: cluster operators provide the Odoo database/admin-password Secrets, an Odoo-scoped Zitadel management credential for bootstrap, and the TLS Secret referenced by chart values; secrets are never committed.
