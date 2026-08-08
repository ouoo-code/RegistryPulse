#!/usr/bin/env sh
set -eu

revision=${1:?usage: rollback.sh <git-revision>}
git diff --quiet || { echo 'Working tree has changes; commit or stash them before rollback.' >&2; exit 2; }
git show --quiet "$revision" || { echo "Unknown git revision: $revision" >&2; exit 2; }
git switch --detach "$revision"
docker compose build api worker frontend
docker compose up -d
docker compose ps
echo "Rollback is complete. Return to the deployment branch with: git switch master"
