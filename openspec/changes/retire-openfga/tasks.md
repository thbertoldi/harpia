## 1. Remove OpenFGA deployment manifests

- [x] 1.1 Delete `deploy/harpia/templates/openfga.yaml` (Helm Deployment+Service + `migrate` initContainer).
- [x] 1.2 Delete `deploy/dev/kind/openfga.yaml`, `deploy/dev/kind/openfga-bootstrap.yaml` (ConfigMap + SA/Role/RoleBinding + bootstrap Job), and `deploy/dev/kind/openfga-model.fga`.
- [x] 1.3 Remove the `openfga:` block from `deploy/harpia/values.yaml:115-120`, `deploy/harpia/values-dev.yaml:21-22`, and `deploy/harpia/values-prod.yaml:25-26`.
- [x] 1.4 Remove the `openfga:` service from `deploy/dev/compose.yaml:89-101`.
- [x] 1.5 Verify: `mise run helm-lint` passes; `grep -rn "openfga" deploy/` returns nothing.

## 2. Remove the OpenFGA Postgres database

- [x] 2.1 Remove the `CREATE DATABASE openfga` + GRANT lines from `deploy/dev/kind/postgres.yaml:83-86` and `deploy/harpia/templates/postgres.yaml:36-39`. Do NOT touch the `harpia`/`zitadel` databases or the Postgres deployment itself. (Also removed from `scripts/sync-dev-postgres-credentials.sh` and deleted `deploy/dev/postgres-init/02-create-openfga.sql`.)
- [x] 2.2 Verify: the shared Postgres still provisions `harpia` and `zitadel`; no `openfga` DB is created.

## 3. Remove dev tooling & scripts

- [x] 3.1 Remove all OpenFGA/FGA wiring from `Tiltfile` (manifest loads ~60-61,76; `openfga`/`openfga-bootstrap` resources ~91-92,139-142; `openfga-port-forward` and `fga-sync` local_resources ~156-160; and the resource deps at ~190,199,201). Ensure no remaining resource lists `fga-sync`/`openfga-port-forward` as a dependency.
- [x] 3.2 Delete the `[tasks.dev-openfga-migrate]` task from `mise.toml:136-138`.
- [x] 3.3 Delete `scripts/sync-fga-config.sh` and `scripts/port-forward-openfga.sh`.
- [x] 3.4 Verify: `grep -rn "fga" Tiltfile mise.toml scripts/` returns nothing.

## 4. Remove dead control-plane config

- [x] 4.1 Delete the `OpenFGAURL`, `OpenFGAStoreID`, `OpenFGAAuthorizationModelID` fields (`control-plane/internal/config/config.go:24-27`) and their env loads (`:52,54-55`).
- [x] 4.2 Delete the aspirational "swap the role-string check for an FGA check_relation" comment at `control-plane/internal/llm_config/handler.go:249-250` (keep the role-string check it describes).
- [x] 4.3 Verify: `cd control-plane && go build ./... && go test ./...` — RLS (`database/postgres_test.go`), identity (`identity/middleware_test.go`), and approvals/elicitations tests stay green.

## 5. Strip generated env keys

- [x] 5.1 Remove `OPENFGA_STORE_ID`/`OPENFGA_AUTHORIZATION_MODEL_ID`/`OPENFGA_API_URL` from `control-plane/.env.local` and `PUBLIC_OPENFGA_*` from `frontend/.env.local`.
- [x] 5.2 Verify: `cd frontend && bun run check` (regenerated `ambient.d.ts` no longer references `PUBLIC_OPENFGA_*`; no new baseline errors); `grep -rn "OPENFGA" control-plane frontend --include=*.local --include=*.ts --include=*.go` returns only intended absences.

## 6. Docs scrub (no runtime impact)

- [x] 6.1 Remove/rewrite OpenFGA mentions in `README.md`, `deploy/harpia/README.md`, `deploy/dev/README.md`, `docs/ISSUES.md`, and `docs/notes/2026-06-16-tenant-llm-config-design.md` to reflect RLS + role checks. Optionally reword the `proto/harpia/llm_config/v1/llm_config.proto:14-20` "tenant.admin" comment (role check, not FGA). (Skipped proto — comment already reads clearly as a role check.) Also scrubbed generated diagrams `docs/architecture.html` and `docs/system-design.html`.
- [x] 6.2 Note: constitution §4/§6/§11 + the ADR map and `docs/adr/README.md` already reflect the retirement — no change needed there. ADR-004 stays frozen history under its banner.

## 7. Final verification

- [x] 7.1 `cd control-plane && go test ./...`; `mise run helm-lint`; `cd frontend && bun run lint && bun run check`.
- [ ] 7.2 `mise run dev` brings the full stack up with **no** OpenFGA resource, and the app still authenticates, isolates tenants, and gates admin/approval actions.
- [x] 7.3 Repo-wide sweep: `grep -rni "openfga\|\bfga\b" --exclude-dir=.worktrees --exclude-dir=.git .` returns nothing but intended doc history.
