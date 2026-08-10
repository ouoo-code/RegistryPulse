#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

docker compose config --quiet
docker compose up -d
trap 'docker compose down' EXIT INT TERM

timeout_seconds=${COMPOSE_SMOKE_TIMEOUT_SECONDS:-120}
deadline=$(( $(date +%s) + timeout_seconds ))
while :; do
  if docker compose ps --status running --services | sort | grep -qx nginx; then
    if curl --fail --silent --show-error --max-time 5 http://127.0.0.1/health >/dev/null; then
      break
    fi
  fi
  if [ "$(date +%s)" -ge "$deadline" ]; then
    docker compose ps
    docker compose logs --no-color api nginx
    echo "Compose smoke test timed out" >&2
    exit 1
  fi
  sleep 3
done

curl --fail --silent --show-error http://127.0.0.1/health >/dev/null
curl --fail --silent --show-error http://127.0.0.1/api/v1/health >/dev/null
curl --fail --silent --show-error http://127.0.0.1/ >/dev/null
curl --fail --silent --show-error http://127.0.0.1:10800/health/live >/dev/null
echo "Compose smoke test passed."
