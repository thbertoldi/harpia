#!/usr/bin/env bash
# Port-forwards the dev OpenFGA service to a host port that is unlikely to
# collide with common local services.
set -euo pipefail

NAMESPACE="${HARPIA_DEV_K8S_NAMESPACE:-default}"
HOST_PORT="${HARPIA_DEV_OPENFGA_PORT:-18086}"
SERVICE_PORT="${HARPIA_DEV_OPENFGA_SERVICE_PORT:-8080}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "ERROR: kubectl is required to port-forward OpenFGA." >&2
  exit 1
fi

if ! [[ "$HOST_PORT" =~ ^[0-9]+$ ]] || (( HOST_PORT < 1 || HOST_PORT > 65535 )); then
  echo "ERROR: HARPIA_DEV_OPENFGA_PORT must be a TCP port number between 1 and 65535." >&2
  exit 1
fi

if ! [[ "$SERVICE_PORT" =~ ^[0-9]+$ ]] || (( SERVICE_PORT < 1 || SERVICE_PORT > 65535 )); then
  echo "ERROR: HARPIA_DEV_OPENFGA_SERVICE_PORT must be a TCP port number between 1 and 65535." >&2
  exit 1
fi

echo "Port-forwarding OpenFGA: http://localhost:${HOST_PORT} -> svc/openfga:${SERVICE_PORT}"
echo "Override the host port with HARPIA_DEV_OPENFGA_PORT if needed."
exec kubectl -n "$NAMESPACE" port-forward svc/openfga "${HOST_PORT}:${SERVICE_PORT}"
