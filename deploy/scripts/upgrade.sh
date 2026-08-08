#!/usr/bin/env sh
set -eu

docker compose config --quiet
docker compose pull postgres redis nginx
docker compose build api worker frontend
docker compose up -d
docker compose ps
