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
k8s_yaml('deploy/dev/kind/frontend.yaml')
k8s_resource('harpia-frontend', port_forwards=['5173:3000'])

# ---- Infrastructure (pre-built images, no build needed) ----
k8s_yaml([
    'deploy/dev/kind/postgres.yaml',
    'deploy/dev/kind/valkey.yaml',
    'deploy/dev/kind/garage.yaml',
    'deploy/dev/kind/zitadel.yaml',
    'deploy/dev/kind/openfga.yaml',
])
