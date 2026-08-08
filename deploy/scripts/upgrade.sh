#!/usr/bin/env sh
set -eu

docker compose config --quiet
docker compose pull
docker compose up -d
docker compose ps
