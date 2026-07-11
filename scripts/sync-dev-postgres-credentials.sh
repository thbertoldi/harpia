#!/usr/bin/env bash
# Reconciles the dev PostgreSQL role password with Secret/postgres-credentials.
#
# PostgreSQL only consumes POSTGRES_PASSWORD during first database
# initialization. If the Kubernetes Secret changes while the PVC is preserved,
# service-to-service clients fail password authentication until the role
# password is updated inside Postgres.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
NAMESPACE="${HARPIA_DEV_K8S_NAMESPACE:-default}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "ERROR: kubectl is required to sync dev PostgreSQL credentials." >&2
  exit 1
fi

"$REPO_ROOT/scripts/ensure-dev-kind-secrets.sh" >/dev/null

DB_USER="$(kubectl -n "$NAMESPACE" get secret postgres-credentials -o jsonpath='{.data.username}' | base64 --decode)"
DB_PASSWORD="$(kubectl -n "$NAMESPACE" get secret postgres-credentials -o jsonpath='{.data.password}' | base64 --decode)"

if [[ -z "$DB_USER" || -z "$DB_PASSWORD" ]]; then
  echo "ERROR: Secret/postgres-credentials must contain non-empty username and password keys." >&2
  exit 1
fi

shell_quote() {
  local value="$1"

  printf "'"
  printf "%s" "$value" | sed "s/'/'\\\\''/g"
  printf "'"
}

REMOTE_DB_USER="$(shell_quote "$DB_USER")"
REMOTE_DB_PASSWORD="$(shell_quote "$DB_PASSWORD")"

echo "Waiting for PostgreSQL pod..."
kubectl -n "$NAMESPACE" wait --for=condition=Ready pod -l app=postgres --timeout=120s >/dev/null

echo "Waiting for PostgreSQL local endpoint..."
kubectl -n "$NAMESPACE" exec deploy/postgres -- sh -ec '
  for _ in $(seq 1 60); do
    if pg_isready -h 127.0.0.1 -U "$POSTGRES_USER" >/dev/null 2>&1; then
      exit 0
    fi
    sleep 1
  done
  echo "ERROR: PostgreSQL did not become ready on 127.0.0.1:5432." >&2
  exit 1
'

echo "Reconciling PostgreSQL role password with Secret/postgres-credentials..."
kubectl -n "$NAMESPACE" exec -i deploy/postgres -- sh -s <<REMOTE
set -eu

DB_USER=$REMOTE_DB_USER
DB_PASSWORD=$REMOTE_DB_PASSWORD

psql -v ON_ERROR_STOP=1 \
  --username "\$POSTGRES_USER" \
  --dbname "\$POSTGRES_DB" \
  -v db_user="\$DB_USER" \
  -v db_password="\$DB_PASSWORD" <<'SQL'
SELECT format('CREATE ROLE %I WITH LOGIN SUPERUSER PASSWORD %L', :'db_user', :'db_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = :'db_user')\gexec
SELECT format('ALTER ROLE %I WITH LOGIN PASSWORD %L', :'db_user', :'db_password')\gexec
SELECT 'CREATE DATABASE zitadel'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'zitadel')\gexec
GRANT ALL PRIVILEGES ON DATABASE zitadel TO :"db_user";
SQL

PGPASSWORD="\$DB_PASSWORD" psql -h postgres -U "\$DB_USER" -d harpia -tAc 'select 1' >/dev/null
REMOTE

echo "PostgreSQL credentials are aligned with Secret/postgres-credentials."
