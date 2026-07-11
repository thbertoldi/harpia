# Harpia Development Environment — Tilt + kind
# Use: mise run dev  (starts tilt up)
#
# Tilt builds backend app images, injects them into k8s manifests,
# applies to the kind cluster, and live-reloads on code changes.

# Odoo is a separately deployable downstream system. It is intentionally not
# part of the default `mise run dev` graph; enable it with `tilt up -- --odoo`.
config.define_bool('odoo', default=False)
config.parse()

local('./scripts/ensure-dev-kind-secrets.sh')

# Fix the kind node's flaky DNS before any pod pulls an image or makes an
# external call (e.g. the plan classifier -> api.deepseek.com). Runs
# synchronously during Tiltfile evaluation, before resources start. Idempotent.
local('./scripts/fix-kind-dns.sh')

# ---- Application Images (built by Tilt with live reload) ----
docker_build(
    'harpia-api',
    context='./control-plane',
    dockerfile='./control-plane/Containerfile',
    # Build the golang `dev` stage (Go toolchain + air + tar) for local dev so
    # live_update can sync source and air rebuilds/restarts in-container. Prod
    # builds omit `target` and get the distroless final stage. The syncs match
    # air's watched dirs (cmd, internal, gen — incl. generated protobuf Go, so
    # proto changes hot-reload too); air handles the rebuild + restart.
    target='dev',
    live_update=[
        sync('./control-plane/cmd', '/app/cmd'),
        sync('./control-plane/internal', '/app/internal'),
        sync('./control-plane/gen', '/app/gen'),
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
    'deploy/dev/kind/garage-bootstrap.yaml',
    'deploy/dev/kind/temporal.yaml',
    'deploy/dev/kind/zitadel.yaml',
    'deploy/dev/kind/zitadel-branding-configmap.yaml',
    'deploy/dev/kind/zitadel-init.yaml',
])

# Temporal dev server (in-memory single binary). The API builds its workflow
# starter at startup, so it must come up after Temporal is ready; the workers
# poll Temporal too. UI on 8233.
k8s_resource('temporal', port_forwards=['8233:8233'], labels=['infra'])

# Garage object-store bootstrap: layout + bucket + access key. Artifact writes
# fail until this runs, so the API and workers depend on it below.
k8s_resource('garage-bootstrap', resource_deps=['garage'], labels=['infra'])

# Temporal workers (built images, reused from harpia-api / harpia-agent).
k8s_yaml('deploy/dev/kind/workers.yaml')

# ---- Host-side dev helpers (Zitadel OIDC, Vite) ----
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

k8s_resource(
    'harpia-api',
    resource_deps=['db-migrate', 'temporal', 'garage-bootstrap'],
    port_forwards=['19080:8080'],
)

# Workers execute plan workflows + agent activities. Go worker needs the DB
# (migrated) and Temporal; the agent worker needs Temporal and (at run time)
# the API for the artifact/LLM/budget internal calls.
k8s_resource(
    'harpia-plan-worker',
    resource_deps=['db-migrate', 'temporal', 'garage-bootstrap'],
    labels=['backend'],
)
k8s_resource(
    'harpia-agent-worker',
    resource_deps=['temporal', 'harpia-api'],
    labels=['backend'],
)

# ---- Optional Odoo downstream stack ----
if config.get('odoo', False):
    # This creates only Odoo-namespace, local-only Secrets and requires an
    # operator-provided Odoo-scoped Zitadel management credential.
    local('./scripts/ensure-dev-kind-secrets.sh --odoo')
    docker_build(
        'harpia-odoo',
        context='.',
        dockerfile='./deploy/odoo/Containerfile',
    )
    k8s_yaml([
        'deploy/dev/kind/odoo-namespace.yaml',
        'deploy/dev/kind/odoo-postgres.yaml',
        'deploy/dev/kind/odoo-oidc-client.yaml',
        'deploy/dev/kind/odoo.yaml',
        'deploy/dev/kind/odoo-ingress.yaml',
    ])
    k8s_resource('odoo-postgres', labels=['odoo'])
    k8s_resource(
        'odoo-oidc-bootstrap',
        resource_deps=['zitadel', 'odoo-postgres'],
        labels=['odoo'],
    )
    k8s_resource(
        'odoo',
        resource_deps=['odoo-postgres', 'odoo-oidc-bootstrap'],
        port_forwards=['18069:8069'],
        labels=['odoo'],
    )

local_resource(
    'zitadel-port-forward',
    serve_cmd='./scripts/port-forward-zitadel.sh',
    resource_deps=['zitadel'],
    deps=['./scripts/port-forward-zitadel.sh'],
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

# Apply the AIUNA Zitadel branding on every `tilt up`. The zitadel-init Job also
# applies it, but Jobs don't re-run once Complete, so a cluster initialized
# before branding existed would keep the default theme. This idempotent
# re-apply guarantees the AIUNA login theme. Needs the localhost:8085
# port-forward and a registered instance.
local_resource(
    'zitadel-branding',
    cmd='./scripts/apply-zitadel-branding.sh',
    resource_deps=['zitadel-register-client', 'zitadel-port-forward'],
    trigger_mode=TRIGGER_MODE_AUTO,
    deps=[
        './scripts/apply-zitadel-branding.sh',
        'deploy/dev/kind/zitadel-branding/apply-branding.sh',
    ],
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
# the OIDC sync resources write frontend/.env.local.
local_resource(
    'vite',
    # PUBLIC_DEV_LOGIN_ENABLED gates the dev-login persona buttons (auth.ts).
    # Set here (not in .env, which is gitignored) so `mise run dev` shows them.
    serve_cmd='cd frontend && PUBLIC_DEV_LOGIN_ENABLED=true bun run dev --host',
    resource_deps=[
        'frontend-install',
        'oidc-sync',
        'zitadel-port-forward',
    ],
    deps=['frontend/.env.local'],
    trigger_mode=TRIGGER_MODE_AUTO,
    readiness_probe=probe(
        period_secs=2,
        http_get=http_get_action(port=5173, path='/'),
    ),
    labels=['frontend'],
)
