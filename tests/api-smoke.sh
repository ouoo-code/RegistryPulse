#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

base_url=${API_BASE_URL:-http://127.0.0.1}
base_url=${base_url%/}
admin_token=${ADMIN_API_TOKEN:-}
curl_args="--fail --silent --show-error --max-time ${API_SMOKE_TIMEOUT_SECONDS:-10}"

request() {
  path=$1
  shift
  # shellcheck disable=SC2086
  curl $curl_args "$@" "$base_url$path"
}

check_envelope() {
  path=$1
  body=$(request "$path")
  printf '%s' "$body" | grep -Fq '"success":true' || {
    echo "API response did not contain success=true: $path" >&2
    printf '%s\n' "$body" >&2
    return 1
  }
  printf '%s\n' "$body"
}

check_envelope /api/v1/health >/dev/null
check_envelope /api/v1/public/summary >/dev/null
check_envelope /api/v1/public/categories >/dev/null
check_envelope /api/v1/public/sources >/dev/null
check_envelope /metrics >/dev/null

if [ -n "$admin_token" ]; then
  auth="Authorization: Bearer $admin_token"
  unauthorized=$(curl --silent --show-error --max-time "${API_SMOKE_TIMEOUT_SECONDS:-10}" -o /dev/null -w '%{http_code}' "$base_url/api/v1/admin/sources")
  [ "$unauthorized" = "401" ] || {
    echo "Expected unauthenticated admin request to return 401, got $unauthorized" >&2
    exit 1
  }
  authorized=$(curl $curl_args -H "$auth" "$base_url/api/v1/admin/sources")
  printf '%s' "$authorized" | grep -Fq '"success":true' || exit 1
fi

echo "API smoke test passed: $base_url"
