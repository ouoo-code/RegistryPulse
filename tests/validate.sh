#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

check() {
  description=$1
  shift
  if "$@"; then
    echo "ok: $description"
  else
    echo "failed: $description" >&2
    exit 1
  fi
}

check "docker is available" command -v docker
check "Compose configuration" docker compose config --quiet
check "nginx configuration exists" test -s deploy/nginx/default.conf
check "nginx API upstream placeholder" grep -Fq 'set $api_upstream api:__API_HTTP_PORT__' deploy/nginx/default.conf
check "nginx API proxy" grep -Fq 'proxy_pass http://$api_upstream' deploy/nginx/default.conf
check "forwarded protocol header" grep -Fq 'proxy_set_header X-Forwarded-Proto' deploy/nginx/default.conf
check "nginx proxy timeout" grep -Fq 'proxy_connect_timeout' deploy/nginx/default.conf
check "registry proxy service" grep -Fq 'proxy:' docker-compose.yml
check "registry proxy binary" grep -Fq 'registry-proxy' Dockerfile
check "registry proxy is read-only" grep -Fq 'push and mutation requests are disabled' backend/internal/proxy/handler.go

for script in deploy/scripts/backup.sh deploy/scripts/restore.sh; do
  check "$script has valid shell syntax" sh -n "$script"
done

check "Makefile invokes deployment validation" grep -Fq 'tests/validate.sh' Makefile
echo "Static deployment validation passed."
