# Harpia Development Environment — Tilt + kind
# Use: mise run dev  (starts tilt up)
#
# Tilt builds app images, injects them into k8s manifests,
# applies to the kind cluster, and live-reloads on code changes.

# ---- Application Images (built by Tilt with live reload) ----
docker_build(
    'harpia-api',
    context='./control-plane',
    dockerfile='./control-plane/Containerfile',
    live_update=[
        sync('./control-plane/cmd', '/app/cmd'),
        sync('./control-plane/internal', '/app/internal'),
        run('cd /app && go build -o /api ./cmd/api/ && /api', trigger=[
            './control-plane/cmd',
            './control-plane/internal',
        ]),
    ],
)
k8s_yaml('deploy/dev/kind/api.yaml')
k8s_resource('harpia-api', port_forwards=['8080:8080'])

docker_build(
    'harpia-agent',
    context='./agent-runtime',
    dockerfile='./agent-runtime/Containerfile',
    live_update=[
        sync('./agent-runtime/src', '/app/src'),
    ],
)
k8s_yaml('deploy/dev/kind/agent.yaml')

docker_build(
    'harpia-frontend',
    context='./frontend',
    dockerfile='./frontend/Containerfile',
    live_update=[
        sync('./frontend/src', '/app/src'),
    ],
)
# Host Vite serves the frontend in dev (picks up frontend/.env.local for OIDC).
# The k8s production build stays available but is not started by default.
k8s_yaml('deploy/dev/kind/frontend.yaml')
k8s_resource('harpia-frontend', auto_init=False)

# ---- Infrastructure (pre-built images, no build needed) ----
k8s_yaml([
    'deploy/dev/kind/postgres.yaml',
    'deploy/dev/kind/valkey.yaml',
    'deploy/dev/kind/garage.yaml',
    'deploy/dev/kind/zitadel.yaml',
    'deploy/dev/kind/zitadel-init.yaml',
    'deploy/dev/kind/openfga.yaml',
])

k8s_resource('zitadel', resource_deps=['postgres'])
k8s_resource('zitadel-register-client', resource_deps=['zitadel'])

# ---- Host-side dev helpers (Zitadel OIDC, Vite) ----
local_resource(
    'zitadel-port-forward',
    serve_cmd='kubectl port-forward svc/zitadel 8085:8080',
    resource_deps=['zitadel'],
    labels=['infra'],
)

local_resource(
    'oidc-sync',
    cmd='./scripts/sync-oidc-config.sh',
    resource_deps=['zitadel-register-client'],
    trigger_mode=TRIGGER_MODE_AUTO,
    deps=['./scripts/sync-oidc-config.sh'],
    labels=['infra'],
)

# Vite loads .env.local at startup only (no HMR for env vars). Restart when
# oidc-sync writes frontend/.env.local or when the sync script changes.
local_resource(
    'vite',
    serve_cmd='cd frontend && bun run dev --host',
    resource_deps=['oidc-sync'],
    deps=['frontend/.env.local'],
    trigger_mode=TRIGGER_MODE_AUTO,
    readiness_probe=probe(
        period_secs=2,
        http_get=http_get_action(port=5173, path='/'),
    ),
    labels=['frontend'],
)
