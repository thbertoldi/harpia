# Development Environment

## Prerequisites
- kind cluster (`kind create cluster --name harpia-dev`)
- kubectl configured for kind

## Start

```bash
mise run dev   # Tilt: infra, Zitadel port-forward, OIDC sync, host Vite
```

Tilt applies `deploy/dev/kind/` (including the Zitadel and OpenFGA bootstrap Jobs),
port-forwards Zitadel to localhost:8085 and OpenFGA to localhost:8086, waits for the
OIDC/FGA ConfigMaps, syncs local `.env.local` files, and starts the Vite dev server
on http://localhost:5173.

Manual apply (without Tilt):

```bash
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
| Zitadel       | http://localhost:8085                 |
| Zitadel Console | http://localhost:8085/ui/console    |
| OpenFGA       | http://localhost:8086                 |
| OpenFGA Playground | http://localhost:8086/playground |

Port-forward commands (only needed outside Tilt):

```bash
kubectl port-forward svc/zitadel 8085:8080 &
kubectl port-forward svc/openfga 8086:8080 &
```

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
