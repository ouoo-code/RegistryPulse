#!/usr/bin/env sh
set -eu

: "${ADMIN_API_TOKEN:?Set ADMIN_API_TOKEN to an administrator bearer token}"
base_url=${BASE_URL:-http://127.0.0.1}
curl --fail --silent --show-error \
  -X POST "${base_url}/api/v1/admin/sources/import" \
  -H "Authorization: Bearer ${ADMIN_API_TOKEN}" \
  -H 'Content-Type: application/json' \
  --data-binary @seed/registry-sources.example.json
printf '\n'
