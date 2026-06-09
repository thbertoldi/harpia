# Investigation: Tilt Dev Startup Failures

## Hand-off Brief

1. **What happened.** Tilt showed two independent startup failures: Vite could not import `@connectrpc/connect`, and `harpia-api` crashed while seeding dev data because Postgres had no `tenants` table.
2. **Where the case stands.** Root causes are confirmed: frontend `node_modules` was stale after dependency changes, and Tilt had no database migration resource before the API started.
3. **What's needed next.** Merge the dev-workflow fix so Tilt installs frontend dependencies, runs Atlas migrations before `harpia-api`, and avoids host port `8080`.

## Case Info

| Field | Value |
| --- | --- |
| Ticket | N/A |
| Date opened | 2026-06-09 |
| Status | Concluded |
| System | Local kind/Tilt dev environment |
| Evidence sources | User logs, repo source, current kind cluster logs, Atlas output |

## Problem Statement

User reported `[500] GET /login` because Vite could not find `@connectrpc/connect`, plus `harpia-api` restarts with `ensure dev tenant: ERROR: relation "tenants" does not exist (SQLSTATE 42P01)` and repeated port-forward failures on host `8080`.

## Evidence Inventory

| Source | Status | Notes |
| --- | --- | --- |
| User logs | Available | Showed missing ConnectRPC package, missing `tenants`, and `8080` bind failures. |
| Frontend package metadata | Available | `frontend/package.json` declares `@connectrpc/connect` and `@connectrpc/connect-web`. |
| Main checkout node_modules | Available | `frontend/node_modules/@connectrpc/connect` was missing before `bun install --frozen-lockfile`. |
| Tiltfile | Available | `harpia-api` had no migration dependency and used `port_forwards=['8080:8080']`. |
| Database migrations | Available | Migrations create `tenants` but there was no `atlas.sum` and no Tilt migration resource. |
| Cluster logs | Available | API logs confirmed healthy seed and 200 health checks after migrations were applied. |

## Confirmed Findings

### Finding 1: Frontend dependency installation was stale

**Evidence:** `frontend/package.json` declares `@connectrpc/connect` and `@connectrpc/connect-web`; `frontend/node_modules/@connectrpc/connect` was missing in the live checkout before running `bun install --frozen-lockfile`.

**Detail:** The source was correct, but Vite ran against a stale dependency tree. Running `bun install --frozen-lockfile` installed the missing ConnectRPC packages.

### Finding 2: API started before schema migrations

**Evidence:** `Tiltfile` started `harpia-api` directly after applying `deploy/dev/kind/api.yaml`; API logs showed `relation "tenants" does not exist`; `database/migrations/000001_initial_schema.sql` creates `tenants`.

**Detail:** The API calls `database.EnsureDevData` at startup, which requires `tenants`. Without a migration resource, a fresh or reset kind database crashes deterministically.

### Finding 3: Atlas could not run without a checksum file

**Evidence:** `atlas migrate status --dir file://database/migrations --url ...` returned `checksum file not found`.

**Detail:** Adding `database/migrations/atlas.sum` made `atlas migrate status` and `atlas migrate apply` succeed.

### Finding 4: Host API port `8080` is already occupied

**Evidence:** User logs showed Tilt could not bind `127.0.0.1:8080`; host inspection showed listeners on `0.0.0.0:8080` and `[::]:8080`.

**Detail:** Keeping the dev API port-forward on `8080` makes Tilt fragile on developer machines. Port `19080` was confirmed free during investigation.

## Deduced Conclusions

### Deduction 1: The frontend 500 and API crash have separate causes

**Based on:** Findings 1 and 2.

**Reasoning:** The missing module is a host dependency problem in Vite. The API crash is a database schema sequencing problem inside kind.

**Conclusion:** Both must be fixed, but neither is caused by the other.

## Source Code Trace

| Element | Detail |
| --- | --- |
| Error origin | `frontend/src/lib/rpc.ts` imports `@connectrpc/connect`; `control-plane/cmd/api/main.go` calls `database.EnsureDevData`. |
| Trigger | `mise run dev` starts host Vite and Tilt starts `harpia-api`. |
| Condition | Host `node_modules` lacks ConnectRPC dependencies; Postgres has not run Atlas migrations; host port `8080` is occupied. |
| Related files | `Tiltfile`, `frontend/vite.config.ts`, `deploy/dev/kind/postgres.yaml`, `database/migrations/*`, `scripts/apply-dev-db-migrations.sh`. |

## Conclusion

**Confidence:** High

The root causes are confirmed and independently reproduced: stale frontend dependencies, missing Atlas migration orchestration, missing `atlas.sum`, and an occupied API port-forward. The current kind cluster was repaired by installing frontend dependencies, switching Postgres to a pgvector-capable image, generating `atlas.sum`, applying migrations, and restarting `harpia-api`.

## Recommended Next Steps

### Fix direction

Commit and merge the dev workflow changes:

- Add a Tilt `frontend-install` resource before Vite.
- Add a Tilt `db-migrate` resource before `harpia-api`.
- Add `database/migrations/atlas.sum`.
- Use a pgvector-enabled dev Postgres image because migration `000002` creates `vector`.
- Move the host API port-forward/proxy from `8080` to `19080`.

### Diagnostic

After merge, run `mise run dev` from a clean kind cluster and confirm:

- `db-migrate` reports no pending errors.
- `harpia-api` logs `dev data ensured`.
- `/api/v1/health` returns 200 through Vite.
- `/login` no longer fails on ConnectRPC import.

## Reproduction Plan

1. Use a checkout with stale `frontend/node_modules` and a kind Postgres database without migrations.
2. Run `mise run dev`.
3. Observe Vite import failure for `@connectrpc/connect` and API startup failure on missing `tenants`.
4. Apply this fix and rerun Tilt; both failures should clear.
