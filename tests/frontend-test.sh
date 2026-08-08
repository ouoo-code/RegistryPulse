#!/usr/bin/env sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root/frontend"
command -v node >/dev/null 2>&1 || { echo 'node is required for frontend verification' >&2; exit 127; }
if [ ! -d node_modules ]; then npm install --no-audit --no-fund; fi
npx tsc --noEmit
npm test -- --run
npm run build
