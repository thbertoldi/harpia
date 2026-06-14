#!/usr/bin/env bash
# Apply Harpia Zitadel branding against a running instance (local port-forward or in-cluster).
#
# Usage:
#   ./scripts/apply-zitadel-branding.sh
#   ZITADEL_URL=http://localhost:8085 PAT_FILE=~/.zitadel-admin.pat ./scripts/apply-zitadel-branding.sh
#
# Defaults target the kind dev stack via kubectl port-forward on localhost:8085.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BRANDING_DIR="$REPO_ROOT/deploy/dev/kind/zitadel-branding"
ZITADEL_URL="${ZITADEL_URL:-http://localhost:8085}"
ZITADEL_HOST="${ZITADEL_HOST:-localhost:8085}"
PAT_FILE="${PAT_FILE:-}"

if [[ ! -x "$BRANDING_DIR/apply-branding.sh" ]]; then
  chmod +x "$BRANDING_DIR/apply-branding.sh"
fi

if [[ -n "${PAT:-}" ]]; then
  :
elif [[ -n "$PAT_FILE" && -s "$PAT_FILE" ]]; then
  PAT="$(tr -d '\r\n' <"$PAT_FILE")"
elif kubectl get deploy zitadel >/dev/null 2>&1; then
  echo "Reading admin PAT from deployment/zitadel..."
  PAT="$(kubectl exec deploy/zitadel -- cat /machinekey/zitadel-admin-sa.pat 2>/dev/null | tr -d '\r\n' || true)"
fi

if [[ -z "${PAT:-}" ]]; then
  echo "ERROR: set PAT, PAT_FILE, or ensure deployment/zitadel exposes /machinekey/zitadel-admin-sa.pat" >&2
  exit 1
fi

export PAT
export ZITADEL_URL ZITADEL_HOST BRANDING_DIR

echo "Applying Harpia branding to $ZITADEL_URL (Host: $ZITADEL_HOST)..."
exec "$BRANDING_DIR/apply-branding.sh"
