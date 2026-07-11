# Harpia Helm Chart

Deploys the Harpia platform stack as Kubernetes resources. The chart covers the
eight services referenced by the dev kind manifests:

| Service   | Template              | Notes                                      |
| --------- | --------------------- | ------------------------------------------ |
| postgres  | `templates/postgres.yaml` | PVC + init script for harpia/zitadel DBs  |
| valkey    | `templates/valkey.yaml`   | Cache / rate limiting                      |
| garage    | `templates/garage.yaml`   | S3-compatible object store                 |
| zitadel   | `templates/zitadel.yaml`  | OIDC identity; machinekey PVC for PAT      |
| api       | `templates/api.yaml`      | Control plane API                          |
| agent     | `templates/agent.yaml`    | Agent runtime                              |
| frontend  | `templates/frontend.yaml` | SvelteKit UI (disabled in `values-dev`)    |

Post-install bootstrap Jobs (`zitadel-init`) remain in
`deploy/dev/kind/` for Tilt/local dev. Helm installs the long-running services;
run those Jobs separately when you need OIDC client bootstrap.

## Prerequisites

Create Secrets before install (not managed by this chart):

| Secret                 | Keys                         | Consumers                    |
| ---------------------- | ---------------------------- | ---------------------------- |
| `postgres-credentials` | `username`, `password`       | postgres, zitadel, api       |
| `zitadel-masterkey`    | `masterkey`                  | zitadel                      |

For local dev, `./scripts/ensure-dev-kind-secrets.sh` creates them.

## Install

```bash
# Default values (single replica, prod-safe auth flags)
helm upgrade --install harpia deploy/harpia/

# Dev overlay (matches kind dev auth + storage; frontend off-cluster)
helm upgrade --install harpia deploy/harpia/ -f deploy/harpia/values-dev.yaml

# Production overlay
helm upgrade --install harpia deploy/harpia/ -f deploy/harpia/values-prod.yaml
```

## Lint / render

```bash
helm lint deploy/harpia/
helm template test deploy/harpia/ > /dev/null
helm template test deploy/harpia/ -f deploy/harpia/values-dev.yaml > /dev/null
```

These commands are also wired into `mise run lint` (`helm lint` + `helm template`).

## Service discovery

In-cluster URLs use Helm release-prefixed Service names, e.g.
`{release}-harpia-postgres:5432`. Application templates derive hostnames from
the release name via `harpia.serviceName` in `_helpers.tpl`.
