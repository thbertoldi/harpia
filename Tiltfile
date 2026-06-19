# Harpia Development Environment — Tilt + kind
# Use: mise run dev  (starts tilt up)
#
# Tilt builds backend app images, injects them into k8s manifests,
# applies to the kind cluster, and live-reloads on code changes.

local('./scripts/ensure-dev-kind-secrets.sh')

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

docker_build(
    'harpia-agent',
    context='.',
    dockerfile='./agent-runtime/Containerfile',
    live_update=[
        sync('./agent-runtime/src', '/app/src'),
        sync('./agents', '/app/agents'),
        run(
            'python -m compileall -q /app/src',
            trigger=['./agent-runtime/src'],
        ),
    ],
)
k8s_resource('harpia-agent', port_forwards=['18000:8000'])

k8s_yaml('deploy/dev/kind/agent.yaml')

# ---- Infrastructure (pre-built images, no build needed) ----
k8s_yaml([
    'deploy/dev/kind/postgres.yaml',
    'deploy/dev/kind/valkey.yaml',
    'deploy/dev/kind/garage.yaml',
    'deploy/dev/kind/zitadel.yaml',
    'deploy/dev/kind/zitadel-branding-configmap.yaml',
    'deploy/dev/kind/zitadel-init.yaml',
    'deploy/dev/kind/openfga.yaml',
    'deploy/dev/kind/openfga-bootstrap.yaml',
])

# ---- Host-side dev helpers (Zitadel OIDC, OpenFGA, Vite) ----
local_resource(
    'postgres-credentials-sync',
    cmd='./scripts/sync-dev-postgres-credentials.sh',
    resource_deps=['postgres'],
    trigger_mode=TRIGGER_MODE_AUTO,
    deps=[
        './scripts/sync-dev-postgres-credentials.sh',
        './scripts/ensure-dev-kind-secrets.sh',
    ],
    labels=['infra'],
)

k8s_resource('zitadel', resource_deps=['postgres-credentials-sync'])
k8s_resource('zitadel-register-client', resource_deps=['zitadel'])
k8s_resource('openfga', resource_deps=['postgres-credentials-sync'])
k8s_resource('openfga-bootstrap', resource_deps=['openfga'])

local_resource(
    'db-migrate',
    cmd='./scripts/apply-dev-db-migrations.sh',
    resource_deps=['postgres-credentials-sync'],
    trigger_mode=TRIGGER_MODE_AUTO,
    deps=[
        './scripts/apply-dev-db-migrations.sh',
        './scripts/sync-dev-postgres-credentials.sh',
        'database/migrations/000001_initial_schema.sql',
        'database/migrations/000002_schema_sync.sql',
        'database/migrations/000003_tenant_rls_hardening.sql',
        'database/migrations/000011_plan_template_executor_metadata.sql',
    ],
    labels=['infra'],
)

k8s_resource('harpia-api', resource_deps=['db-migrate'], port_forwards=['19080:8080'])

local_resource(
    'zitadel-port-forward',
    serve_cmd='./scripts/port-forward-zitadel.sh',
    resource_deps=['zitadel'],
    deps=['./scripts/port-forward-zitadel.sh'],
    labels=['infra'],
)

local_resource(
    'openfga-port-forward',
    serve_cmd='./scripts/port-forward-openfga.sh',
    resource_deps=['openfga'],
    deps=['./scripts/port-forward-openfga.sh'],
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

local_resource(
    'fga-sync',
    cmd='./scripts/sync-fga-config.sh',
    resource_deps=['openfga', 'oidc-sync'],
    trigger_mode=TRIGGER_MODE_AUTO,
    deps=['./scripts/sync-fga-config.sh'],
    labels=['infra'],
)

local_resource(
    'frontend-install',
    cmd='cd frontend && bun install --frozen-lockfile',
    trigger_mode=TRIGGER_MODE_AUTO,
    deps=['frontend/package.json', 'frontend/bun.lock'],
    labels=['frontend'],
)

# Vite loads .env.local at startup only (no HMR for env vars). Restart after
# the OIDC/FGA sync resources write frontend/.env.local.
local_resource(
    'vite',
    serve_cmd='cd frontend && bun run dev --host',
    resource_deps=[
        'frontend-install',
        'oidc-sync',
        'fga-sync',
        'zitadel-port-forward',
        'openfga-port-forward',
    ],
    deps=['frontend/.env.local'],
    trigger_mode=TRIGGER_MODE_AUTO,
    readiness_probe=probe(
        period_secs=2,
        http_get=http_get_action(port=5173, path='/'),
    ),
    labels=['frontend'],
)
