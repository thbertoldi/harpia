# Development Environment

## Prerequisites
- kind cluster (`kind create cluster --name harpia-dev`)
- kubectl configured for kind

## Start

```bash
kubectl apply -f deploy/dev/k3s/
kubectl apply -f deploy/dev/kind/
```

Apply the Zitadel init Job after Zitadel is ready:

```bash
kubectl wait --for=condition=Ready pod -l app=zitadel --timeout=120s
kubectl create -f deploy/dev/kind/zitadel-init.yaml
kubectl logs job/zitadel-register-client -f
```

## Access

| Service       | URL                                   |
| ------------- | ------------------------------------- |
| Frontend      | http://localhost:5173                 |
| Zitadel       | http://localhost:8085                 |
| Zitadel Console | http://localhost:8085/ui/console    |

Port-forward commands:

```bash
kubectl port-forward svc/zitadel 8085:8080 &
kubectl port-forward svc/harpia-frontend 5173:3000 &
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

Copy the generated `Client ID` to `frontend/src/lib/auth.ts`.

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
9. Copy the **Client ID** and update `frontend/src/lib/auth.ts`:

```typescript
const ZITADEL_CONFIG = {
  issuer: "http://localhost:8085",
  clientId: "<YOUR_CLIENT_ID>",
  redirectUri: "http://localhost:5173/auth/callback",
  scope: "openid profile email",
};
```
