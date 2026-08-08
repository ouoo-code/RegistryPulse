#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
docker compose config --quiet

test -s deploy/nginx/default.conf
grep -Fq 'set $api_upstream api:8080' deploy/nginx/default.conf
grep -Fq 'proxy_pass http://$api_upstream' deploy/nginx/default.conf
grep -Fq 'proxy_set_header X-Forwarded-Proto' deploy/nginx/default.conf
grep -Fq 'proxy_connect_timeout' deploy/nginx/default.conf

for script in deploy/scripts/backup.sh deploy/scripts/restore.sh; do
  test -x "$script" || chmod +x "$script"
  sh -n "$script"
done

grep -Fq 'docker compose config' Makefile
grep -Fq 'tests/validate.sh' Makefile
echo "Static deployment validation passed."
