# Harpia Development Environment
# Use: tilt up
#
# Tilt starts infrastructure via podman-compose. Application services
# (api, agent, frontend) are run locally for hot reload:
#   mise run dev-api     — Go API server on :8080
#   mise run dev-agent   — Python agent runtime on :8000
#   mise run dev-web     — Svelte frontend on :5173

# ---- Infrastructure (PostgreSQL + Valkey + Garage + Zitadel + OpenFGA) ----
docker_compose('./deploy/dev/compose.yaml')

# ---- Application images (built on change, run locally) ----
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
