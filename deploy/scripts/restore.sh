#!/usr/bin/env sh
set -eu
if [ "$#" -ne 1 ]; then echo "usage: $0 <backup-directory>" >&2; exit 2; fi
dir=$1
if [ ! -f "$dir/postgres.sql" ]; then echo "postgres.sql not found" >&2; exit 1; fi
if ! docker compose ps --services --filter status=running | grep -qx postgres; then
  echo "postgres service is not running; start Compose before restoring" >&2
  exit 1
fi
echo "This replaces database contents. Type RESTORE to continue:"
read answer
if [ "$answer" != "RESTORE" ]; then echo "cancelled"; exit 1; fi
docker compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < "$dir/postgres.sql"
