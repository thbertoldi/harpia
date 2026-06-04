# Harpia Development Environment
# Use: tilt up

# ---- Go API Server ----
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
        run('cd /app && go build -o /api ./cmd/api/ && /api', trigger='./control-plane/cmd'),
        run('cd /app && go build -o /api ./cmd/api/ && /api', trigger='./control-plane/internal'),
    ],
)

# ---- Agent Runtime ----
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

# ---- Frontend ----
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

# ---- PostgreSQL (dev) ----
docker_compose('./deploy/dev/compose.yaml')

# Port forwards (for services built by tilt's docker_build)
k8s_resource('harpia-api', port_forwards=['8080:8080'])
k8s_resource('harpia-frontend', port_forwards=['5173:3000'])
