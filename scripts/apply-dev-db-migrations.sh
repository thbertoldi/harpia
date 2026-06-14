#!/usr/bin/env bash
# Applies Atlas migrations to the dev kind PostgreSQL service before harpia-api starts.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MIGRATIONS_DIR="$REPO_ROOT/database/migrations"
PORT="${HARPIA_DEV_DB_PORT:-15432}"
NAMESPACE="${HARPIA_DEV_K8S_NAMESPACE:-default}"
LOG_FILE="${TMPDIR:-/tmp}/harpia-postgres-port-forward.log"
PF_PID=""

cleanup() {
  if [[ -n "$PF_PID" ]] && kill -0 "$PF_PID" 2>/dev/null; then
    kill "$PF_PID" 2>/dev/null || true
    wait "$PF_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

if ! command -v atlas >/dev/null 2>&1; then
  echo "ERROR: atlas is required. Run 'mise install' so the dev toolchain is available." >&2
  exit 1
fi

"$REPO_ROOT/scripts/sync-dev-postgres-credentials.sh" >/dev/null

if [[ -n "${HARPIA_DEV_DATABASE_URL:-}" ]]; then
  DATABASE_URL="$HARPIA_DEV_DATABASE_URL"
else
  DB_USER="$(kubectl -n "$NAMESPACE" get secret postgres-credentials -o jsonpath='{.data.username}' | base64 --decode)"
  DB_PASSWORD="$(kubectl -n "$NAMESPACE" get secret postgres-credentials -o jsonpath='{.data.password}' | base64 --decode)"
  DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${PORT}/harpia?sslmode=disable"
fi

echo "Waiting for PostgreSQL pod..."
kubectl wait --for=condition=Ready pod -l app=postgres --timeout=120s >/dev/null

echo "Port-forwarding PostgreSQL on localhost:${PORT}..."
rm -f "$LOG_FILE"
kubectl port-forward svc/postgres "${PORT}:5432" >"$LOG_FILE" 2>&1 &
PF_PID=$!

READY=false
for _ in $(seq 1 60); do
  if atlas migrate status --dir "file://${MIGRATIONS_DIR}" --url "$DATABASE_URL" >/dev/null 2>&1; then
    READY=true
    break
  fi

  if ! kill -0 "$PF_PID" 2>/dev/null; then
    echo "ERROR: postgres port-forward exited before migrations could run." >&2
    cat "$LOG_FILE" >&2 || true
    exit 1
  fi

  sleep 1
done

if [[ "$READY" != "true" ]]; then
  echo "ERROR: timed out waiting for PostgreSQL migration endpoint on localhost:${PORT}." >&2
  cat "$LOG_FILE" >&2 || true
  exit 1
fi

echo "Applying database migrations..."
atlas migrate apply --dir "file://${MIGRATIONS_DIR}" --url "$DATABASE_URL"
