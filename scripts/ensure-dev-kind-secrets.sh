#!/usr/bin/env bash
# Creates the local kind Secrets required by the dev manifests without writing
# generated secret values to the repository.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_OVERRIDES_FILE="$REPO_ROOT/deploy/dev/kind/secrets.local.env"
OVERRIDES_FILE="${HARPIA_DEV_KIND_SECRETS_FILE:-$DEFAULT_OVERRIDES_FILE}"

random_hex() {
  local bytes="$1"

  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex "$bytes"
    return
  fi

  od -An -N "$bytes" -tx1 /dev/urandom | tr -d ' \n'
  printf '\n'
}

random_b64() {
  local bytes="$1"

  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 "$bytes"
    return
  fi

  dd if=/dev/urandom bs="$bytes" count=1 2>/dev/null | base64 | tr -d '\n'
  printf '\n'
}

if [[ -f "$OVERRIDES_FILE" ]]; then
  set -a
  # shellcheck source=/dev/null
  source "$OVERRIDES_FILE"
  set +a
fi

NAMESPACE="${HARPIA_DEV_K8S_NAMESPACE:-default}"
POSTGRES_USERNAME="${HARPIA_DEV_POSTGRES_USERNAME:-harpia}"
POSTGRES_PASSWORD="${HARPIA_DEV_POSTGRES_PASSWORD:-$(random_hex 24)}"
ZITADEL_KEY="${HARPIA_DEV_ZITADEL_KEY:-$(random_hex 16)}"
# Garage S3 credentials. Defaults are fixed (not random) because the same key
# must be imported into Garage by garage-bootstrap.yaml AND consumed by the API
# / workers from this secret. The access key id must be GK + 24 hex chars.
GARAGE_ACCESS_KEY="${HARPIA_DEV_GARAGE_ACCESS_KEY:-GKa1b2c3d4e5f6a7b8c9d0e1f2}"
GARAGE_SECRET_KEY="${HARPIA_DEV_GARAGE_SECRET_KEY:-b1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6}"
LLM_KEK_V1_B64="${HARPIA_DEV_LLM_KEK_V1_B64:-$(random_b64 32)}"
LLM_KEK_ACTIVE="${HARPIA_DEV_LLM_KEK_ACTIVE:-v1}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "ERROR: kubectl is required to create dev kind Secrets." >&2
  exit 1
fi

require_secret_keys() {
  local name="$1"
  shift

  local missing=()
  local key
  for key in "$@"; do
    local value
    value="$(kubectl -n "$NAMESPACE" get secret "$name" -o "jsonpath={.data.${key}}" 2>/dev/null || true)"
    if [[ -z "$value" ]]; then
      missing+=("$key")
    fi
  done

  if (( ${#missing[@]} > 0 )); then
    echo "ERROR: Secret/${name} exists but is missing key(s): ${missing[*]}" >&2
    echo "Delete or repair the Secret before starting dev infra." >&2
    exit 1
  fi
}

ensure_secret() {
  local name="$1"
  shift

  if kubectl -n "$NAMESPACE" get secret "$name" >/dev/null 2>&1; then
    echo "Secret/${name} already exists in namespace ${NAMESPACE}; keeping existing values."
    require_secret_keys "$name" "$@"
    return
  fi

  case "$name" in
    postgres-credentials)
      kubectl -n "$NAMESPACE" create secret generic "$name" \
        --from-literal=username="$POSTGRES_USERNAME" \
        --from-literal=password="$POSTGRES_PASSWORD" >/dev/null
      ;;
    zitadel-masterkey)
      kubectl -n "$NAMESPACE" create secret generic "$name" \
        --from-literal=masterkey="$ZITADEL_KEY" >/dev/null
      ;;
    garage-credentials)
      kubectl -n "$NAMESPACE" create secret generic "$name" \
        --from-literal=accessKey="$GARAGE_ACCESS_KEY" \
        --from-literal=secretKey="$GARAGE_SECRET_KEY" >/dev/null
      ;;
    harpia-llm-kek)
      kubectl -n "$NAMESPACE" create secret generic "$name" \
        --from-literal=v1_b64="$LLM_KEK_V1_B64" \
        --from-literal=active="$LLM_KEK_ACTIVE" >/dev/null
      ;;
    *)
      echo "ERROR: unknown Secret ${name}" >&2
      exit 1
      ;;
  esac

  echo "Created Secret/${name} in namespace ${NAMESPACE}."
}

ensure_secret postgres-credentials username password
ensure_secret zitadel-masterkey masterkey
ensure_secret garage-credentials accessKey secretKey
ensure_secret harpia-llm-kek v1_b64 active
