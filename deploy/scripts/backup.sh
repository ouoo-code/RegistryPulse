#!/usr/bin/env sh
set -eu

if ! docker compose ps --services --filter status=running | grep -qx postgres; then
  echo "postgres service is not running; start Compose before backing up" >&2
  exit 1
fi

stamp=$(date -u +%Y-%m-%d_%H-%M-%S)
out="${BACKUP_DIR:-backups}/$stamp"
mkdir -p "$out"
tmp="$out/postgres.sql.tmp"

# Read credentials from the container environment so the host shell does not
# need to source .env and secrets are not echoed by the script.
docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --no-privileges' > "$tmp"
mv "$tmp" "$out/postgres.sql"
cp .env.example "$out/env.example"
printf '%s\n' "Backup created at $out; secrets were not copied."
