# Development Environment

## Prerequisites
- kind cluster (`kind create cluster --name harpia-dev`)
- kubectl configured for kind

## Start

```bash
mise run dev   # Tilt: infra, Zitadel port-forward, OIDC sync, host Vite
```

Tilt applies `deploy/dev/kind/`, runs database migrations, bootstraps Zitadel
and OpenFGA, port-forwards the API to localhost:19080, Zitadel to
localhost:8085, and OpenFGA to localhost:8086, waits for the OIDC/FGA ConfigMaps,
syncs local `.env.local` files, installs frontend dependencies when needed, and
starts the Vite dev server on http://localhost:5173. Tilt creates missing
dev-only Kubernetes Secrets before applying manifests.

The dev kind environment intentionally does not create a `harpia-frontend` pod.
Frontend development runs on the host through Vite so HMR and synced
`frontend/.env.local` values stay fast and visible. A dev-cluster frontend
container path can be added later when it is needed for prod-parity testing.
If an older dev cluster already has the removed frontend deployment, delete the
stale resources once:

```bash
kubectl delete deployment,service harpia-frontend --ignore-not-found
```

Manual apply (without Tilt):

```bash
./scripts/ensure-dev-kind-secrets.sh
kubectl apply -f deploy/dev/kind/postgres.yaml
./scripts/apply-dev-db-migrations.sh
kubectl apply -f deploy/dev/kind/
kubectl wait --for=condition=Ready pod -l app=zitadel --timeout=120s
kubectl wait --for=condition=Available deployment/openfga --timeout=120s
kubectl apply -f deploy/dev/kind/zitadel-init.yaml
kubectl logs job/zitadel-register-client -f
./scripts/sync-oidc-config.sh
kubectl logs job/openfga-bootstrap -f
./scripts/sync-fga-config.sh
```

## Access

| Service       | URL                                   |
| ------------- | ------------------------------------- |
| Frontend      | http://localhost:5173 (host Vite via Tilt) |
| API           | http://localhost:19080                |
| Zitadel       | http://localhost:8085                 |
| Zitadel Console | http://localhost:8085/ui/console    |
| OpenFGA       | http://localhost:8086                 |
| OpenFGA Playground | http://localhost:8086/playground |

Port-forward commands (only needed outside Tilt):

```bash
kubectl port-forward svc/zitadel 8085:8080 &
kubectl port-forward svc/openfga 8086:8080 &
```

## Dev Secrets

The kind manifests expect two Kubernetes Secrets in the active namespace:

| Secret | Keys | Consumers |
| ------ | ---- | --------- |
| `postgres-credentials` | `username`, `password` | Postgres, Zitadel, API, OpenFGA |
| `zitadel-masterkey` | `masterkey` | Zitadel |

Run `./scripts/ensure-dev-kind-secrets.sh` before applying `deploy/dev/kind/`.
The helper creates missing Secrets with generated dev values and keeps existing
Secrets unchanged on later runs.

To choose local values, create ignored file `deploy/dev/kind/secrets.local.env`:

```bash
HARPIA_DEV_POSTGRES_USERNAME=harpia
HARPIA_DEV_POSTGRES_PASSWORD=<url-safe password>
HARPIA_DEV_ZITADEL_KEY=<32-character value>
```

For an existing dev cluster created before these Secrets existed, seed
`secrets.local.env` with the current Postgres role password before the first
apply, then rotate using the steps below. Generating a new Secret for an old PVC
does not change the password stored inside Postgres.

The generated values live only in Kubernetes Secret objects unless you export
them yourself. If a dev database PVC matters, back up both Secrets outside the
repository before deleting the cluster or namespace:

```bash
kubectl get secret postgres-credentials -o yaml > /secure/path/postgres-credentials.yaml
kubectl get secret zitadel-masterkey -o yaml > /secure/path/zitadel-masterkey.yaml
```

### Rotation

Postgres password rotation without data loss:

1. Generate a new URL-safe password.
2. Connect with the current password and run `ALTER ROLE` for the Secret's
   `username`.
3. Apply `Secret/postgres-credentials` with the same `username` and new
   `password`.
4. Restart consumers: `kubectl rollout restart deploy/postgres deploy/zitadel deploy/harpia-api deploy/openfga`.
5. Run `./scripts/apply-dev-db-migrations.sh` and verify API/OpenFGA/Zitadel
   readiness.

Updating only the Secret does not change the existing Postgres role password;
that order causes clients to fail authentication.

Zitadel master key rotation is not a normal password rotation. It protects data
encrypted in the Zitadel database, so changing `Secret/zitadel-masterkey` while
keeping the same database can make existing data unreadable. For a dev reset,
delete the Zitadel/Postgres PVCs and the Secret, then rerun the setup helper. To
preserve data, keep the old key, take a database backup, follow Zitadel's
upstream master key migration procedure, update the Secret only after the data
has been migrated, and restart Zitadel once verification passes.

## Zitadel

**Admin credentials:**
- URL: http://localhost:8085/ui/console
- Username: admin@harpia.local
- Password: HarpiaAdmin1!

### OIDC Client Registration

The `zitadel-init.yaml` Job registers the Harpia Web client as **Native** (PKCE, no secret) so
`http://localhost:5173` loopback redirects stay OIDC-compliant in dev without
`noneCompliant` / `RedirectUris.HttpOnlyForWeb` noise in Job logs.

Check its logs:

```bash
kubectl logs job/zitadel-register-client
```

Tilt syncs the generated `Client ID` to `frontend/.env.local`. Outside Tilt, run
`./scripts/sync-oidc-config.sh` after the Job publishes `ConfigMap/harpia-oidc-config`.

#### Manual Registration

If the auto-registration Job fails, create the client manually:

1. Open http://localhost:8085/ui/console
2. Login with `admin@harpia.local` / `HarpiaAdmin1!`
3. Navigate to **Projects** → **Harpia** (create if it doesn't exist)
4. Under the project, click **New Application**
5. Fill in:
   - Name: `Harpia Web`
   - Type: **Native** (allows `http://localhost` loopback redirects without OIDC compliance warnings in dev)
   - Authentication Method: **None** (PKCE)
6. Add redirect URI: `http://localhost:5173/auth/callback`
7. Add post-logout redirect URI: `http://localhost:5173`
8. Click **Create**
9. Publish the **Client ID** through the same ConfigMap used by Tilt, then sync it:

```bash
kubectl create configmap harpia-oidc-config \
  --from-literal=clientId="<YOUR_CLIENT_ID>" \
  --from-literal=issuer="http://localhost:8085" \
  --dry-run=client -o yaml | kubectl apply -f -
./scripts/sync-oidc-config.sh
```

## OpenFGA

The `openfga-bootstrap.yaml` Job creates or reuses the `Harpia` OpenFGA store,
writes the dev authorization model from `deploy/dev/kind/openfga-model.fga`, and
publishes `storeId` and `authorizationModelId` to `ConfigMap/harpia-fga-config`.
The Job is idempotent after a successful run: if the ConfigMap points at an
existing store/model pair, the Job reuses it instead of writing another immutable
authorization model.

Check its logs:

```bash
kubectl logs job/openfga-bootstrap
```

Tilt syncs the generated IDs to `frontend/.env.local` and
`control-plane/.env.local`. Outside Tilt, run:

```bash
./scripts/sync-fga-config.sh
```

Verify the model with the OpenFGA CLI:

```bash
STORE_ID=$(kubectl get configmap harpia-fga-config -o jsonpath='{.data.storeId}')
fga model list --api-url http://localhost:8086 --store-id "$STORE_ID"
```
