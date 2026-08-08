#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
docker compose up -d
cleanup() { docker compose down; }
trap cleanup EXIT INT TERM

deadline=$(( $(date +%s) + ${E2E_START_TIMEOUT_SECONDS:-120} ))
while ! curl --fail --silent http://127.0.0.1/health >/dev/null 2>&1; do
  if [ "$(date +%s)" -ge "$deadline" ]; then
    docker compose ps
    docker compose logs --no-color api nginx
    exit 1
  fi
  sleep 2
done

cd tests/e2e
npm install --no-audit --no-fund
npx playwright install chromium
npm test
