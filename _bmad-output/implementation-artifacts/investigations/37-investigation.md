# Investigation 37: OpenFGA Authorization Model Bootstrap

Status: Concluded
Date: 2026-06-09
Input: GitHub issue #37, `feat(authz): OpenFGA authorization model + bootstrap Job`

## Hand-off Brief

OpenFGA was present as a dev service, but no dev bootstrap path loaded a Harpia authorization model or surfaced the resulting IDs. The implemented fix adds a dev-only model, idempotent bootstrap Job, ConfigMap handoff, host sync script, and Tilt wiring while keeping runtime authorization checks out of scope for #38. #38 is now unblocked with respect to model/store availability, but still needs the application enforcement adapter and tuple-writing flow.

## Case Info

- Scope: dev/bootstrap infrastructure only.
- Worktree: `/home/thbertoldi/harpia/.worktrees/issue-37-openfga-bootstrap`
- Branch: `issue-37-openfga-bootstrap`
- Stronghold: issue #37 requires an OpenFGA model, idempotent bootstrap Job, `ConfigMap/harpia-fga-config`, and sync script.

## Evidence Inventory

- Confirmed: ADR-006 classifies Authorization (ReBAC) as a generic subdomain to adopt via OpenFGA, not build inside the core domain (`docs/adr/ADR-006-domain-driven-design.md:31`-`33`).
- Confirmed: ADR-004 assigns AuthZ responsibility to OpenFGA and describes nested resource traversal for tenant/workspace/task-style relationships (`docs/adr/ADR-004-identity-and-access.md:28`-`45`).
- Confirmed: the existing dev OpenFGA manifest only migrates and runs OpenFGA; it has no model-loading container, Job, or ConfigMap publication (`deploy/dev/kind/openfga.yaml:19`-`42`).
- Confirmed: the existing control-plane config had Zitadel/Garage/Valkey/Temporal configuration but no OpenFGA store/model configuration before this change (`control-plane/internal/config/config.go:14`-`22` now shows the added OpenFGA fields).
- Confirmed: the implemented model covers `tenant`, `workspace`, `task`, `subtask`, `agent_type`, `tool`, and `user` with owner/member/assignee/viewer relations plus the issue-required reviewer/engineer mappings (`deploy/dev/kind/openfga-model.fga:1`-`54`).
- Confirmed: the bootstrap Job publishes `storeId` and `authorizationModelId` to `ConfigMap/harpia-fga-config` (`deploy/dev/kind/openfga-bootstrap.yaml:517`-`520`, `deploy/dev/kind/openfga-bootstrap.yaml:565`-`570`).
- Confirmed: Tilt now applies the bootstrap manifest, orders the OpenFGA resources, port-forwards OpenFGA on localhost:8086, and runs `fga-sync` before Vite (`Tiltfile:48`-`107`).

## Hypotheses

1. Status: Confirmed. OpenFGA had no loaded model because there was no bootstrap job/API call in the kind path.
   Resolution: `openfga-bootstrap` now creates/reuses the store, writes the model, verifies stored ConfigMap pointers on rerun, and publishes IDs.

2. Status: Confirmed. OIDC and FGA sync scripts would conflict if both wrote `frontend/.env.local` as full-file replacements.
   Resolution: `sync-oidc-config.sh` and `sync-fga-config.sh` write only their owned keys and preserve unrelated env entries.

## Final Conclusion

Confidence: Medium. The evidence is strong for repository structure, generated manifests, shell syntax, JSON validity, and Kubernetes client validation. Live acceptance still depends on a running kind cluster and OpenFGA accepting the model at runtime; no local `fga` CLI is installed in this environment for model semantic validation.

## Fix Direction Implemented

- Adopted a dev/bootstrap boundary instead of embedding OpenFGA concerns into domain packages.
- Added `deploy/dev/kind/openfga-model.fga` as the human-readable model source.
- Added `deploy/dev/kind/openfga-bootstrap.yaml` with a ConfigMap-backed model payload and an idempotent bootstrap Job.
- Added `scripts/sync-fga-config.sh` to mirror the OIDC sync flow for `frontend/.env.local` and `control-plane/.env.local`.
- Updated `Tiltfile` so OpenFGA bootstrap and sync run in the dev workflow.
- Added OpenFGA config fields to `control-plane/internal/config/config.go` for #38 to consume through configuration instead of direct environment reads.

## Verification Plan

Completed:

- `bash -n scripts/sync-fga-config.sh scripts/sync-oidc-config.sh`
- `gofmt -w control-plane/internal/config/config.go`
- `git diff --check`
- `kubectl apply --dry-run=client --validate=false -f deploy/dev/kind/openfga-bootstrap.yaml`
- `kubectl apply --dry-run=client --validate=false -f deploy/dev/kind/openfga.yaml`
- `kubectl apply --dry-run=client --validate=false -f deploy/dev/kind/`
- `diff -u -B deploy/dev/kind/openfga-model.fga <(yq -r 'select(.kind == "ConfigMap" and .metadata.name == "harpia-openfga-model") | .data."openfga-model.fga"' deploy/dev/kind/openfga-bootstrap.yaml)`
- `yq -r 'select(.kind == "ConfigMap" and .metadata.name == "harpia-openfga-model") | .data."openfga-model.json"' deploy/dev/kind/openfga-bootstrap.yaml | jq . >/dev/null`
- `env GOCACHE=/tmp/harpia-go-cache go test ./...` in `control-plane`

Not completed:

- `fga model validate --file deploy/dev/kind/openfga-model.fga` because the `fga` CLI is not installed in this environment.
- Live `fga model list --store-id ...` because no live kind/OpenFGA cluster was exercised in this worktree.

## #38 Boundary

#38 is unblocked with respect to #37: a Harpia OpenFGA store/model can now be bootstrapped and its IDs are surfaced for host-side development. #38 still needs runtime authorization checks, tuple-writing semantics, API integration, and tests; none of those checks were implemented here.
