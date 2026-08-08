#!/usr/bin/env sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root/backend"
if command -v go >/dev/null 2>&1; then
  go test ./...
  go vet ./...
  go build ./...
else
  command -v docker >/dev/null 2>&1 || { echo 'go or docker is required for Go verification' >&2; exit 127; }
  image=${GO_VERIFY_IMAGE:-golang:1.22-alpine}
  docker run --rm -v "$PWD:/src" -w /src "$image" sh -lc 'go_bin=go; command -v go >/dev/null 2>&1 || go_bin=/usr/local/go/bin/go; "$go_bin" test ./... && "$go_bin" vet ./... && "$go_bin" build ./...'
fi
