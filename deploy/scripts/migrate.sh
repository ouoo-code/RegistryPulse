#!/usr/bin/env sh
set -eu

docker compose up -d postgres redis api
echo "PostgreSQL migrations are applied by the API before it starts."
docker compose ps postgres redis api
