#!/bin/sh
# This script is invoked only by the Odoo-scoped bootstrap Job. It has no
# Harpia client/config references and receives no credential in the web Pod.
set -eu

CURL_BIN="${CURL_BIN:-curl}"
KUBECTL_BIN="${KUBECTL_BIN:-kubectl}"

required() {
  value="$1"
  name="$2"
  [ -n "$value" ] || { echo "ERROR: $name is required" >&2; exit 1; }
}

required "$ZITADEL_ISSUER" ZITADEL_ISSUER
required "$ZITADEL_MANAGEMENT_API_URL" ZITADEL_MANAGEMENT_API_URL
required "$ZITADEL_MANAGEMENT_TOKEN_FILE" ZITADEL_MANAGEMENT_TOKEN_FILE
required "$ODOO_REDIRECT_URI" ODOO_REDIRECT_URI
required "$ODOO_LOGOUT_URI" ODOO_LOGOUT_URI
ZITADEL_INTERNAL_ISSUER="${ZITADEL_INTERNAL_ISSUER:-$ZITADEL_ISSUER}"

for i in $(seq 1 120); do
  "$CURL_BIN" -sf "$ZITADEL_INTERNAL_ISSUER/.well-known/openid-configuration" >/dev/null && break
  sleep 2
done
"$CURL_BIN" -sf "$ZITADEL_INTERNAL_ISSUER/.well-known/openid-configuration" >/dev/null

PAT="$(cat "$ZITADEL_MANAGEMENT_TOKEN_FILE")"
api() {
  method="$1"
  path="$2"
  body="${3:-}"
  if [ -n "$body" ]; then
    "$CURL_BIN" -sfS -X "$method" "$ZITADEL_MANAGEMENT_API_URL$path" \
      -H "Authorization: Bearer $PAT" -H "Content-Type: application/json" -d "$body"
  else
    "$CURL_BIN" -sfS -X "$method" "$ZITADEL_MANAGEMENT_API_URL$path" \
      -H "Authorization: Bearer $PAT"
  fi
}

json_value() {
  # alpine/k8s ships python3; use it instead of regex over JSON text. The Zitadel
  # search responses nest the target key under "result", so a recursive lookup is
  # a faithful replacement for the previous leftmost-string sed match. Missing
  # keys print nothing and exit 0 so the caller's `required` guard fires.
  python3 -c '
import json, sys


def find(obj, key):
    if isinstance(obj, dict):
        if key in obj:
            return obj[key]
        for value in obj.values():
            found = find(value, key)
            if found is not None:
                return found
    elif isinstance(obj, list):
        for value in obj:
            found = find(value, key)
            if found is not None:
                return found
    return None


result = find(json.load(sys.stdin), sys.argv[1])
if result is not None:
    print(result)
' "$1"
}

project_search='{"queries":[{"nameQuery":{"name":"'"$ZITADEL_PROJECT_NAME"'","method":"TEXT_QUERY_METHOD_EQUALS"}}]}'
project_response="$(api POST /management/v1/projects/_search "$project_search")"
project_id="$(printf '%s' "$project_response" | json_value id)"
if [ -z "$project_id" ]; then
  project_id="$(api POST /management/v1/projects '{"name":"'"$ZITADEL_PROJECT_NAME"'"}' | json_value id)"
fi
required "$project_id" project_id

app_search='{"queries":[{"nameQuery":{"name":"'"$ZITADEL_APPLICATION_NAME"'","method":"TEXT_QUERY_METHOD_EQUALS"}}]}'
app_response="$(api POST "/management/v1/projects/$project_id/apps/_search" "$app_search")"
app_id="$(printf '%s' "$app_response" | json_value id)"
oidc_config='{"redirectUris":["'"$ODOO_REDIRECT_URI"'"],"responseTypes":["OIDC_RESPONSE_TYPE_CODE"],"grantTypes":["OIDC_GRANT_TYPE_AUTHORIZATION_CODE"],"appType":"OIDC_APP_TYPE_WEB","authMethodType":"OIDC_AUTH_METHOD_TYPE_BASIC","postLogoutRedirectUris":["'"$ODOO_LOGOUT_URI"'"],"version":"OIDC_VERSION_1_0","devMode":false,"accessTokenType":"OIDC_TOKEN_TYPE_BEARER","idTokenUserinfoAssertion":true}'

# Odoo is a confidential, server-side relying party, so the client is a web app
# authenticated with a client secret (HTTP Basic at the token endpoint). The
# Management API never returns an existing secret on GET, so a value published on
# a previous bootstrap run is preserved here; it is Odoo client metadata read
# only by Odoo's server-side handler, so it lives in the same ConfigMap.
existing_client_secret="$("$KUBECTL_BIN" -n "$POD_NAMESPACE" get configmap "$OIDC_METADATA_CONFIGMAP" -o jsonpath='{.data.clientSecret}' 2>/dev/null || true)"

if [ -z "$app_id" ]; then
  app_response="$(api POST "/management/v1/projects/$project_id/apps/oidc" '{"name":"'"$ZITADEL_APPLICATION_NAME"'",'"${oidc_config#\{}")"
  app_id="$(printf '%s' "$app_response" | json_value appId)"
  client_id="$(printf '%s' "$app_response" | json_value clientId)"
  client_secret="$(printf '%s' "$app_response" | json_value clientSecret)"
else
  api PUT "/management/v1/projects/$project_id/apps/$app_id/oidc_config" "$oidc_config" >/dev/null
  app_response="$(api GET "/management/v1/projects/$project_id/apps/$app_id")"
  client_id="$(printf '%s' "$app_response" | json_value clientId)"
  client_secret="$existing_client_secret"
  if [ -z "$client_secret" ]; then
    # First reconciliation of an existing app with no recorded secret: rotate it
    # and republish below. The rotated value is the only one Odoo will hold.
    client_secret="$(api POST "/management/v1/projects/$project_id/apps/$app_id/oidc_client_secret" | json_value clientSecret)"
  fi
fi
required "$app_id" app_id
required "$client_id" client_id
required "$client_secret" client_secret

"$KUBECTL_BIN" -n "$POD_NAMESPACE" create configmap "$OIDC_METADATA_CONFIGMAP" \
  --from-literal=clientId="$client_id" \
  --from-literal=clientSecret="$client_secret" \
  --from-literal=issuer="$ZITADEL_ISSUER" \
  --from-literal=authorizationEndpoint="$ZITADEL_ISSUER/oauth/v2/authorize" \
  --from-literal=tokenEndpoint="$ZITADEL_INTERNAL_ISSUER/oauth/v2/token" \
  --from-literal=jwksUri="$ZITADEL_INTERNAL_ISSUER/oauth/v2/keys" \
  --from-literal=redirectUri="$ODOO_REDIRECT_URI" \
  --from-literal=postLogoutRedirectUri="$ODOO_LOGOUT_URI" \
  --dry-run=client -o yaml | "$KUBECTL_BIN" -n "$POD_NAMESPACE" apply -f -
