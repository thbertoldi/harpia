# Harpia Development Environment
# Default: podman compose for infra + local app services
# Optional: k3s for full k8s parity (requires sudo)
# Use: mise run dev

# ---- Infrastructure (podman compose) ----
docker_compose('./deploy/dev/compose.yaml')

# ---- Application images (built on change) ----
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
)

docker_build(
    'harpia-agent',
    context='./agent-runtime',
    dockerfile='./agent-runtime/Containerfile',
    only=[
        './agent-runtime/src',
        './agent-runtime/pyproject.toml',
    ],
)

docker_build(
    'harpia-frontend',
    context='./frontend',
    dockerfile='./frontend/Containerfile',
    only=[
        './frontend/src',
        './frontend/package.json',
    ],
)
