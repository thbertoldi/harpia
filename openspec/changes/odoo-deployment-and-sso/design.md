## Context

The Platform Constitution §15 establishes Aiuna as the operations layer over a downstream system of record, with Odoo first. Odoo owns detailed record management and its business data; Harpia remains the front door for governed, high-level operations. This change supplies the first prerequisite: a self-hosted Odoo 19 Community system that is independently deployable and entered through Zitadel SSO. It does not create an Odoo connector, mirror records, or let Harpia mutate Odoo.

The repository already deploys Harpia, Zitadel, and their shared PostgreSQL service through `deploy/harpia/`, and kind development uses `deploy/dev/kind/zitadel.yaml` plus `zitadel-init.yaml` to register Harpia's OIDC client. Odoo must not be folded into that chart or reuse its database, OIDC client, or sessions. The new chart therefore has an independent lifecycle and namespace. Harpia must continue to deploy and run with no Odoo release installed.

## Goals / Non-Goals

**Goals:**
- Deploy Odoo 19 Community Edition as its own Helm release in namespace `odoo`.
- Run a dedicated PostgreSQL instance/database and persistent Odoo filestore for that release.
- Build an Odoo image with an immutable pin of OCA `auth_oidc` and the repository-owned `harpia_auth_zitadel` addon.
- Register and configure a distinct Zitadel OIDC client for Odoo, with Odoo-only redirect/logout URIs and client configuration.
- Permit only a pre-existing Odoo user with a verified email claim to complete OIDC login; deny unknown OIDC subjects and all JIT user creation.
- Publish the Odoo UI through a Traefik TLS Ingress, without committing TLS private material.
- Make Odoo available to kind/Tilt development as an explicitly optional resource with a focused smoke-test path.

**Non-Goals:**
- Provisioning users from Zitadel through Harpia into Odoo; that is the separate `downstream-user-provisioning` change.
- An Odoo MCP server, Harpia connector, ExecutorInstallation, credential propagation, or live record reads; that is `odoo-connector-foundation`.
- Downstream Odoo mutations, compensations, approvals, or Plan integration; that is `governed-downstream-mutations`.
- Custom Odoo CRM, ERP, or other business modules.
- Sharing a database, OIDC client, bearer token, browser session, cookie, or PostgreSQL credential between Odoo and Harpia/Zitadel.
- Managing certificate issuance or a production DNS provider. The cluster operator supplies the TLS Secret named by chart values.

## Decisions

### D1: Separate optional Helm release and namespace

`deploy/odoo/` is an application chart released independently from `deploy/harpia/` into `odoo`. It renders its Namespace (unless an operator opts to use an existing namespace), Odoo Deployment/Service, PostgreSQL Deployment/Service, PVCs, Ingress, and bootstrap configuration. `deploy/harpia/` receives no Odoo templates, values, dependencies, or runtime configuration. Consequently, uninstalling or never installing Odoo cannot block Harpia.

### D2: Isolated Odoo persistence

Odoo runs against a dedicated PostgreSQL service and database in the `odoo` namespace, with credentials read from an Odoo-only Secret. Its filestore is mounted on a distinct `ReadWriteOnce` PVC at Odoo's filestore path. The PostgreSQL PVC and filestore PVC use `Recreate`/single-writer deployment semantics so an Odoo replica cannot corrupt a shared volume. No Harpia or Zitadel Pod mounts or connects to either store.

### D3: Image-owned authentication addons with immutable OCA provenance

`deploy/odoo/Containerfile` extends the official Odoo 19 Community image. It installs OCA `auth_oidc` from the compatible `OCA/server-auth` 19.0 line at an immutable commit recorded in the Containerfile (and exposes no floating branch/tag as the production default), then copies the first-party addon into a dedicated additional-addons directory. The Odoo configuration enables both addons and places the first-party path before no untrusted writable addon path. Build metadata records the OCA repository/ref so an operator can audit the image.

The exact immutable OCA commit is selected and reviewed when the image is implemented; the acceptance criterion is a verified 19.0-compatible commit, not an unpinned branch tip. This change does not introduce a package manager or runtime download of addons.

### D4: Separate OIDC relying party and idempotent bootstrap

Odoo and Harpia are separate OIDC relying parties. An idempotent Odoo bootstrap Job uses an operator-supplied, Odoo-scoped Zitadel management credential to create or reconcile an Odoo-only client and publishes only its non-secret OIDC metadata for the Odoo Deployment. The Job never reads, alters, or republishes `harpia-oidc-config`; it does not expose a management credential to the Odoo web container.

The Odoo client uses authorization-code OIDC flow and has only the Odoo values-configured redirect and post-logout redirect URIs. Its issuer, client ID, and Odoo provider configuration are injected into Odoo without sharing Harpia's browser session or access/ID tokens. Redirect URLs, issuer URL, and the client registration protocol are values so development and production hosts can differ without code changes.

### D5: Hardening addon denies JIT and requires verified identity

The repository-owned `harpia_auth_zitadel` module depends on OCA `auth_oidc` and constrains its successful OIDC authentication path. It accepts a login only when the upstream token contains `email_verified: true` and resolves to an already-existing Odoo user through the identity linkage that the future provisioning flow will establish. It must reject an absent/unverified email claim and an unknown identity before any Odoo user creation path can run. It must not fall back to matching or creating an account solely from an unverified email address.

The addon preserves Odoo's normal authorization checks after authentication. It contains no Harpia connector code, Harpia bearer-token handling, or business modules. Its operator-visible denial strings are extracted for English and Brazilian Portuguese (`pt_BR`) Odoo translations.

### D6: TLS ends at Traefik ingress

The chart emits a `networking.k8s.io/v1` Ingress with `ingressClassName: traefik`, a configured Odoo host, and a `spec.tls` entry referencing a pre-provisioned Secret. The application Service remains `ClusterIP`; no Odoo or database NodePort is emitted. The default production-oriented values require an explicit ingress host and TLS Secret rather than silently rendering an HTTP public endpoint. The kind/Tilt path may use a port-forward for local iteration, but it does not relax the chart's TLS-ingress requirement.

## Risks / Trade-offs

- **[OCA/Odoo 19 compatibility]** → OCA 19.0 support and the selected immutable commit must be proven with an image build and Odoo module-install test before release. The image must fail its build rather than silently using a floating or incompatible addon revision.
- **[OIDC extension-point drift]** → `harpia_auth_zitadel` is covered by focused tests for known and unknown identities, verified and unverified email claims, and JIT denial. Tests use the pinned `auth_oidc` authentication extension point so an upstream change cannot silently bypass the hardening module.
- **[Zitadel bootstrap privilege]** → The bootstrap Job uses a separately supplied credential limited to Odoo client/project management where Zitadel permits it, mounts it only in that Job, and never mounts it in Odoo. It is a release prerequisite and is not stored in chart values or Git.
- **[First release has no provisioned Odoo users]** → This is intentional. Until `downstream-user-provisioning` lands, an operator must create the matching Odoo user through the supported administrative path before that user can sign in. Unknown identities receive a denial, not JIT access.
- **[Persistent single-writer storage]** → The initial filestore uses `ReadWriteOnce`, so Odoo is one replica. Horizontal scaling requires shared filestore storage and is outside this slice.
- **[Optional deployment drift]** → Helm render/lint and a kind smoke test verify that Odoo can install independently, while a separate Harpia-only check confirms no Odoo release is required.
