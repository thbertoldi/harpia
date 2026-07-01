#!/usr/bin/env bash
# Apply AIUNA Zitadel branding against a running instance (local port-forward or in-cluster).
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
elif kubectl get pvc zitadel-machinekey >/dev/null 2>&1; then
  echo "Reading admin PAT from the zitadel-machinekey PVC..."
  # The dev Zitadel image is distroless (no shell/cat), so `kubectl exec` into it
  # can't read the PAT. Instead mount the machinekey PVC into a short-lived,
  # shell-capable pod and read the PAT that Zitadel's first-instance bootstrap
  # wrote to /machinekey/zitadel-admin-sa.pat.
  # Create the reader pod, wait for it to finish, then read its stdout via
  # `kubectl logs`. We deliberately avoid `kubectl run --rm -i`: attaching mixes
  # teardown/attach noise into stdout and corrupts the captured PAT.
  READER_POD="zitadel-pat-reader"
  READER_OVERRIDES='{"spec":{"volumes":[{"name":"mk","persistentVolumeClaim":{"claimName":"zitadel-machinekey"}}],"containers":[{"name":"reader","image":"alpine/k8s:1.31.0","imagePullPolicy":"IfNotPresent","command":["sh","-c","cat /machinekey/zitadel-admin-sa.pat"],"volumeMounts":[{"name":"mk","mountPath":"/machinekey"}]}]}}'
  kubectl delete pod "$READER_POD" --ignore-not-found >/dev/null 2>&1 || true
  kubectl run "$READER_POD" --restart=Never \
    --image=alpine/k8s:1.31.0 --image-pull-policy=IfNotPresent \
    --overrides="$READER_OVERRIDES" >/dev/null 2>&1 || true
  kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "pod/$READER_POD" --timeout=60s >/dev/null 2>&1 || true
  PAT="$(kubectl logs "$READER_POD" 2>/dev/null | tr -d '\r\n' || true)"
  kubectl delete pod "$READER_POD" --ignore-not-found >/dev/null 2>&1 || true
fi

if [[ -z "${PAT:-}" ]]; then
  echo "ERROR: set PAT, PAT_FILE, or ensure the zitadel-machinekey PVC holds /machinekey/zitadel-admin-sa.pat" >&2
  exit 1
fi

export PAT
export ZITADEL_URL ZITADEL_HOST BRANDING_DIR

echo "Applying AIUNA branding to $ZITADEL_URL (Host: $ZITADEL_HOST)..."
exec "$BRANDING_DIR/apply-branding.sh"
