#!/usr/bin/env bash
# Port-forwards the dev OpenFGA service. The script verifies the host port
# before invoking kubectl and reuses an already healthy forward when one exists.
set -euo pipefail

NAMESPACE="${HARPIA_DEV_K8S_NAMESPACE:-default}"
HOST_PORT="${HARPIA_DEV_OPENFGA_PORT:-18086}"
SERVICE_PORT="${HARPIA_DEV_OPENFGA_SERVICE_PORT:-8080}"
RECHECK_SECONDS="${HARPIA_DEV_PORT_FORWARD_RECHECK_SECONDS:-5}"
HEALTH_URL="http://localhost:${HOST_PORT}/healthz"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "ERROR: kubectl is required to port-forward OpenFGA." >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "ERROR: curl is required to verify the OpenFGA port-forward." >&2
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

if ! [[ "$RECHECK_SECONDS" =~ ^[0-9]+$ ]] || (( RECHECK_SECONDS < 1 )); then
  echo "ERROR: HARPIA_DEV_PORT_FORWARD_RECHECK_SECONDS must be a positive integer." >&2
  exit 1
fi

openfga_healthy() {
  curl -fsS --max-time 2 "$HEALTH_URL" 2>/dev/null | grep -q '"SERVING"'
}

port_accepts_tcp() {
  timeout 1 bash -c ":</dev/tcp/127.0.0.1/${HOST_PORT}" >/dev/null 2>&1
}

show_listener() {
  if command -v ss >/dev/null 2>&1; then
    ss -ltnp "sport = :${HOST_PORT}" >&2 || true
  fi
}

if openfga_healthy; then
  echo "OpenFGA is already available at ${HEALTH_URL}; reusing the existing listener."
  while openfga_healthy; do
    sleep "$RECHECK_SECONDS"
  done
  echo "ERROR: existing OpenFGA listener on localhost:${HOST_PORT} stopped responding." >&2
  exit 1
fi

if port_accepts_tcp; then
  echo "ERROR: localhost:${HOST_PORT} is already in use but is not serving OpenFGA." >&2
  echo "Stop the stale listener below, or set HARPIA_DEV_OPENFGA_PORT to a free port before starting Tilt." >&2
  show_listener
  exit 1
fi

echo "Port-forwarding OpenFGA: http://localhost:${HOST_PORT} -> svc/openfga:${SERVICE_PORT}"
echo "Override the host port with HARPIA_DEV_OPENFGA_PORT if needed."
exec kubectl -n "$NAMESPACE" port-forward svc/openfga "${HOST_PORT}:${SERVICE_PORT}"
