#!/bin/sh
# Apply Harpia branding to a Zitadel instance via Admin + Assets APIs.
# Idempotent: safe to re-run after asset or palette changes.
#
# Required env:
#   PAT          — IAM_OWNER personal access token
# Optional env:
#   ZITADEL_URL  — base URL (default http://zitadel:8080)
#   ZITADEL_HOST — Host header for external domain (default localhost:8085)
#   BRANDING_DIR — directory with logo.svg and icon.svg (default /branding)
set -eu

ZITADEL_URL="${ZITADEL_URL:-http://zitadel:8080}"
ZITADEL_HOST="${ZITADEL_HOST:-localhost:8085}"
BRANDING_DIR="${BRANDING_DIR:-/branding}"

PRIMARY_COLOR="#C8920F"
BACKGROUND_COLOR="#121318"
FONT_COLOR="#F5F2EB"
WARN_COLOR="#CD3D56"

if [ -z "${PAT:-}" ]; then
  echo "ERROR: PAT is required" >&2
  exit 1
fi

LOGO_FILE="$BRANDING_DIR/logo.svg"
ICON_FILE="$BRANDING_DIR/icon.svg"
for asset in "$LOGO_FILE" "$ICON_FILE"; do
  if [ ! -f "$asset" ]; then
    echo "ERROR: branding asset not found: $asset" >&2
    exit 1
  fi
done

api_json() {
  method=$1
  path=$2
  body=${3:-}
  if [ -n "$body" ]; then
    curl -sfS -X "$method" "$ZITADEL_URL$path" \
      -H "Authorization: Bearer $PAT" \
      -H "Content-Type: application/json" \
      -H "Host: $ZITADEL_HOST" \
      -d "$body"
  else
    curl -sfS -X "$method" "$ZITADEL_URL$path" \
      -H "Authorization: Bearer $PAT" \
      -H "Host: $ZITADEL_HOST"
  fi
}

upload_instance_asset() {
  endpoint=$1
  file=$2
  label=$3
  echo "  uploading $label..."
  status=$(curl -sS -o /tmp/zitadel-asset-response.txt -w "%{http_code}" \
    -X POST "$ZITADEL_URL/assets/v1/instance/policy/label/$endpoint" \
    -H "Authorization: Bearer $PAT" \
    -H "Host: $ZITADEL_HOST" \
    -F "file=@$file" || true)
  case "$status" in
    200|201|204)
      echo "  $label uploaded (HTTP $status)"
      ;;
    *)
      echo "ERROR: $label upload failed with HTTP $status" >&2
      cat /tmp/zitadel-asset-response.txt >&2 || true
      exit 1
      ;;
  esac
}

LABEL_POLICY_BODY=$(cat <<EOF
{
  "primaryColor": "$PRIMARY_COLOR",
  "backgroundColor": "$BACKGROUND_COLOR",
  "fontColor": "$FONT_COLOR",
  "warnColor": "$WARN_COLOR",
  "primaryColorDark": "$PRIMARY_COLOR",
  "backgroundColorDark": "$BACKGROUND_COLOR",
  "fontColorDark": "$FONT_COLOR",
  "warnColorDark": "$WARN_COLOR",
  "hideLoginNameSuffix": true,
  "disableWatermark": true,
  "themeMode": "THEME_MODE_DARK"
}
EOF
)

echo "[branding] Uploading Harpia assets to instance label policy..."
upload_instance_asset "logo" "$LOGO_FILE" "logo"
upload_instance_asset "logo/dark" "$LOGO_FILE" "logo (dark)"
upload_instance_asset "icon" "$ICON_FILE" "icon"
upload_instance_asset "icon/dark" "$ICON_FILE" "icon (dark)"

echo "[branding] Updating instance label policy palette..."
api_json PUT /admin/v1/policies/label "$LABEL_POLICY_BODY" >/dev/null
echo "  label policy updated"

echo "[branding] Activating instance label policy..."
api_json POST /admin/v1/policies/label/_activate '{}' >/dev/null
echo "  label policy activated"

POLICY_CHECK=$(api_json GET /admin/v1/policies/label)
if ! printf '%s' "$POLICY_CHECK" | grep -Fq "$PRIMARY_COLOR"; then
  echo "ERROR: label policy verification failed — primary color not present" >&2
  exit 1
fi
if ! printf '%s' "$POLICY_CHECK" | grep -Fq "$BACKGROUND_COLOR"; then
  echo "ERROR: label policy verification failed — background color not present" >&2
  exit 1
fi

echo "[branding] Harpia theme applied (primary=$PRIMARY_COLOR background=$BACKGROUND_COLOR text=$FONT_COLOR)"
