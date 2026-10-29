#!/usr/bin/env bash
# Walks the API up to the camera step: create → PUT photo to S3 → check photo
# → create liveness session. Usage: scripts/smoke.sh selfie.jpg
set -euo pipefail

API=${API:-http://localhost:8080}
PHOTO=${1:-testdata/face.jpg}

if [[ ! -f $PHOTO ]]; then
  echo "No photo at $PHOTO. Pass a JPEG of one face: scripts/smoke.sh path/to/photo.jpg" >&2
  exit 2
fi
for tool in curl jq; do
  command -v "$tool" >/dev/null || { echo "$tool is required" >&2; exit 2; }
done

body=$(mktemp)
trap 'rm -f "$body"' EXIT

call() {
  local method=$1 url=$2
  shift 2
  curl -sS -o "$body" -w '%{http_code} %{time_total}s' -X "$method" "$@" "$url"
}

expect() {
  local want=$1 got=$2 label=$3
  printf '%-28s %s\n' "$label" "$got"
  if [[ ${got%% *} != "$want" ]]; then
    echo "  expected $want; response body:" >&2
    jq . "$body" 2>/dev/null >&2 || cat "$body" >&2
    exit 1
  fi
}

expect 200 "$(call GET "$API/healthz")" "health"

expect 201 "$(call POST "$API/api/verifications")" "create verification"
id=$(jq -r .id "$body")
upload_url=$(jq -r .uploadUrl "$body")
content_type=$(jq -r '.uploadHeaders["Content-Type"]' "$body")
echo "  id: $id"

expect 200 "$(call PUT "$upload_url" -H "Content-Type: $content_type" --data-binary @"$PHOTO")" "upload photo to S3"

status=$(call POST "$API/api/verifications/$id/photo")
if [[ ${status%% *} == 422 ]]; then
  printf '%-28s %s\n' "check photo" "$status"
  echo "  photo rejected: $(jq -r '.reasons | join(", ")' "$body")" >&2
  echo "  (the API works; try a clearer, front-facing photo of one person)" >&2
  exit 1
fi
expect 200 "$status" "check photo"

expect 200 "$(call POST "$API/api/verifications/$id/liveness-session")" "create liveness session"
echo "  session: $(jq -r .sessionId "$body") (attempt $(jq -r '"\(.attempt) of \(.maxAttempts)"' "$body"))"

expect 409 "$(call POST "$API/api/verifications/$id/complete")" "complete (expect not finished)"
echo "  code: $(jq -r .code "$body")"

expect 200 "$(call GET "$API/api/verifications/$id")" "get verification"
jq '{id, status, livenessAttempts, photoUrl: (.photoUrl | .[0:60] + "...")}' "$body"

echo
echo "OK up to the camera step. The session expires unused in 3 minutes, at no cost."
