#!/usr/bin/env bash
# Port-forwards the dev Zitadel service. Zitadel's configured external issuer
# uses localhost:8085, so this script verifies that port before invoking
# kubectl and reuses an already healthy forward when one exists.
set -euo pipefail

NAMESPACE="${HARPIA_DEV_K8S_NAMESPACE:-default}"
HOST_PORT=8085
SERVICE_PORT=8080
RECHECK_SECONDS="${HARPIA_DEV_PORT_FORWARD_RECHECK_SECONDS:-5}"
HEALTH_URL="http://localhost:${HOST_PORT}/.well-known/openid-configuration"
HOST_HEADER="localhost:${HOST_PORT}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "ERROR: kubectl is required to port-forward Zitadel." >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "ERROR: curl is required to verify the Zitadel port-forward." >&2
  exit 1
fi

if ! [[ "$RECHECK_SECONDS" =~ ^[0-9]+$ ]] || (( RECHECK_SECONDS < 1 )); then
  echo "ERROR: HARPIA_DEV_PORT_FORWARD_RECHECK_SECONDS must be a positive integer." >&2
  exit 1
fi

zitadel_healthy() {
  curl -fsS --max-time 2 -H "Host: ${HOST_HEADER}" "$HEALTH_URL" 2>/dev/null \
    | grep -q '"issuer":"http://localhost:8085"'
}

port_accepts_tcp() {
  timeout 1 bash -c ":</dev/tcp/127.0.0.1/${HOST_PORT}" >/dev/null 2>&1
}

show_listener() {
  if command -v ss >/dev/null 2>&1; then
    ss -ltnp "sport = :${HOST_PORT}" >&2 || true
  fi
}

if zitadel_healthy; then
  echo "Zitadel is already available at ${HEALTH_URL}; reusing the existing listener."
  while zitadel_healthy; do
    sleep "$RECHECK_SECONDS"
  done
  echo "ERROR: existing Zitadel listener on localhost:${HOST_PORT} stopped responding." >&2
  exit 1
fi

if port_accepts_tcp; then
  echo "ERROR: localhost:${HOST_PORT} is already in use but is not serving Zitadel." >&2
  echo "Stop the stale listener below, then restart Tilt." >&2
  show_listener
  exit 1
fi

echo "Port-forwarding Zitadel: http://localhost:${HOST_PORT} -> svc/zitadel:${SERVICE_PORT}"
exec kubectl -n "$NAMESPACE" port-forward svc/zitadel "${HOST_PORT}:${SERVICE_PORT}"
