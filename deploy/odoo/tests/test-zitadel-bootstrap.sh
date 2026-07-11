#!/bin/sh
# Lightweight isolated test harness for the chart's bootstrap script. It mocks
# Zitadel Management API calls and kubectl, including a second reconciliation.
set -eu

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
state="$tmp/state"
log="$tmp/log"
printf 'first\n' > "$state"
printf 'management-token\n' > "$tmp/token"

cat > "$tmp/curl" <<'EOF'
#!/bin/sh
set -eu
url=""
for arg in "$@"; do
  case "$arg" in http://*|https://*) url="$arg" ;; esac
done
method=GET
body=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -X) method="$2"; shift 2 ;;
    -d) body="$2"; shift 2 ;;
    *) shift ;;
  esac
done
case "$url" in
  */.well-known/openid-configuration) printf '{}' ;;
  */projects/_search) printf '{"result":[]}' ;;
  */projects)
    if [ "$method" = POST ]; then printf '{"id":"odoo-project"}'; else printf '{}'; fi ;;
  */apps/_search)
    if [ "$(cat "$MOCK_STATE")" = first ]; then printf '{"result":[]}'
    else printf '{"result":[{"id":"odoo-app","clientId":"odoo-client-id"}]}'
    fi ;;
  */apps/oidc)
    printf 'created:%s\n' "$body" >> "$MOCK_LOG"
    printf 'reconciled\n' > "$MOCK_STATE"
    printf '{"appId":"odoo-app","clientId":"odoo-client-id"}' ;;
  */oidc_config)
    printf 'updated:%s\n' "$body" >> "$MOCK_LOG" ;;
  */apps/odoo-app) printf '{"id":"odoo-app","clientId":"odoo-client-id"}' ;;
  *) printf '{}' ;;
esac
EOF
chmod +x "$tmp/curl"

cat > "$tmp/kubectl" <<'EOF'
#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$MOCK_LOG"
case "$*" in
  *'create configmap'*) printf 'apiVersion: v1\nkind: ConfigMap\n' ;;
  *) cat >/dev/null || true ;;
esac
EOF
chmod +x "$tmp/kubectl"

run_bootstrap() {
  MOCK_STATE="$state" MOCK_LOG="$log" CURL_BIN="$tmp/curl" KUBECTL_BIN="$tmp/kubectl" \
    POD_NAMESPACE=odoo ZITADEL_ISSUER=https://zitadel.example.test \
    ZITADEL_INTERNAL_ISSUER=https://zitadel-internal.example.test \
    ZITADEL_MANAGEMENT_API_URL=https://zitadel-api.example.test \
    ZITADEL_MANAGEMENT_TOKEN_FILE="$tmp/token" ZITADEL_PROJECT_NAME=Odoo \
    ZITADEL_APPLICATION_NAME='Harpia Odoo' \
    ODOO_REDIRECT_URI=https://odoo.example.test/auth_oauth/signin \
    ODOO_LOGOUT_URI=https://odoo.example.test OIDC_METADATA_CONFIGMAP=odoo-oidc-config \
    sh "$root/scripts/bootstrap-zitadel-client.sh"
}

run_bootstrap
run_bootstrap

if grep -F 'harpia-oidc-config' "$log" >/dev/null; then
  echo 'bootstrap must not mutate Harpia OIDC configuration' >&2
  exit 1
fi
