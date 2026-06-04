# Harpia Development Environment — k3s + Tilt
# Use: tilt up
#
# Requires k3s running locally. All services deploy to k3s.
# Port forwards: API on :8080, Frontend on :5173
#
# App services use docker_build with live_update for hot reload.
# Infra services are deployed from static k3s manifests.

# ---- Infrastructure (k3s manifests) ----
k8s_yaml([
    'deploy/dev/k3s/postgres.yaml',
    'deploy/dev/k3s/valkey.yaml',
    'deploy/dev/k3s/garage.yaml',
    'deploy/dev/k3s/zitadel.yaml',
    'deploy/dev/k3s/openfga.yaml',
])

# ---- Control Plane (Go) ----
docker_build(
    'harpia-api',
    context='./control-plane',
    dockerfile='./control-plane/Containerfile',
    only=[
        './control-plane/cmd',
        './control-plane/internal',
        './control-plane/go.mod',
        './control-plane/go.sum',
    ],
    live_update=[
        sync('./control-plane/cmd', '/app/cmd'),
        sync('./control-plane/internal', '/app/internal'),
        run('cd /app && go build -o /api ./cmd/api/ && /api', trigger=[
            './control-plane/cmd',
            './control-plane/internal',
        ]),
    ],
)
k8s_yaml('deploy/dev/k3s/api.yaml')
k8s_resource('harpia-api', port_forwards=['8080:8080'])

# ---- Agent Runtime (Python) ----
docker_build(
    'harpia-agent',
    context='./agent-runtime',
    dockerfile='./agent-runtime/Containerfile',
    only=[
        './agent-runtime/src',
        './agent-runtime/pyproject.toml',
    ],
    live_update=[
        sync('./agent-runtime/src', '/app/src'),
    ],
)
k8s_yaml('deploy/dev/k3s/agent.yaml')

# ---- Frontend (SvelteKit) ----
docker_build(
    'harpia-frontend',
    context='./frontend',
    dockerfile='./frontend/Containerfile',
    only=[
        './frontend/src',
        './frontend/package.json',
    ],
    live_update=[
        sync('./frontend/src', '/app/src'),
    ],
)
k8s_yaml('deploy/dev/k3s/frontend.yaml')
k8s_resource('harpia-frontend', port_forwards=['5173:3000'])
