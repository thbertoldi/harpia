# Odoo 19 Community deployment

This chart deploys an **optional**, standalone Odoo 19 Community release. It
does not modify `deploy/harpia/`; Harpia remains deployable with no Odoo
release installed. Odoo is placed in its own `odoo` namespace, uses only its
own PostgreSQL instance and filestore PVC, and exposes only a TLS-protected
Traefik Ingress. Both Services are `ClusterIP` only.

## Prerequisites

Create these Odoo-only Secrets in the target namespace before install. Do not
reuse Harpia, Zitadel, or another Odoo release's Secrets.

| Secret | Keys | Used by |
| --- | --- | --- |
| `odoo-postgres-credentials` | `username`, `password` | Odoo PostgreSQL and Odoo only |
| `odoo-admin-password` | `admin-password` | Odoo database manager |
| `odoo-zitadel-management` | `token` | Odoo bootstrap Job only |
| TLS Secret named by `ingress.tlsSecret` | `tls.crt`, `tls.key` | Traefik Ingress |

The management token must be an operator-provided, Odoo-scoped Zitadel
Management API credential. The chart mounts it only in the bootstrap Job; the
Odoo Deployment does not mount or reference it. The Job publishes only the
Odoo client ID and non-secret provider metadata to an Odoo-named ConfigMap.
It never reads or mutates `harpia-oidc-config`.

## Install and remove

Set a real, HTTPS Odoo host, its pre-created TLS Secret, Zitadel issuer and
Management API URL, and the exact Odoo callback URLs. The callback must end in
`/auth_oauth/signin`.

```bash
helm upgrade --install odoo deploy/odoo --namespace odoo --create-namespace \
  -f deploy/odoo/values-prod.yaml \
  --set ingress.host=odoo.example.com \
  --set ingress.tlsSecret=odoo-example-com-tls \
  --set oidc.issuer=https://zitadel.example.com \
  --set oidc.managementApiUrl=https://zitadel.example.com \
  --set oidc.redirectUri=https://odoo.example.com/auth_oauth/signin \
  --set oidc.postLogoutRedirectUri=https://odoo.example.com

helm uninstall odoo --namespace odoo
```

TLS is mandatory: the chart fails if `ingress.host` or `ingress.tlsSecret` is
empty and never emits an HTTP fallback. The cluster operator owns DNS and
certificate issuance.

## Zitadel sign-in policy

The image pins OCA `auth_oidc` to commit
`2a908b49821c743b85d4d9beac9baa42c379c2d6` from
`https://github.com/OCA/server-auth.git`, then loads the image-owned
`harpia_auth_zitadel` addon. It accepts only a Zitadel ID token with
`email_verified: true` whose `sub` has already been linked to an Odoo user.
Unknown subjects, missing/false verification claims, and email-only matches
are denied without creating a user or linkage.

Before a person can sign in, an Odoo administrator must pre-provision that
person's Odoo user and set its OAuth provider to `Zitadel` with `oauth_uid`
equal to the Zitadel subject. There is no just-in-time login. This is not the
future Zitadel → Harpia → Odoo user-provisioning workflow; that remains out of
scope for this chart.

## Verification

```bash
helm lint deploy/odoo/
helm template odoo deploy/odoo/ -f deploy/odoo/values-dev.yaml
deploy/odoo/tests/test-zitadel-bootstrap.sh
```
