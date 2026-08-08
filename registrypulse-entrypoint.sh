#!/bin/sh
set -eu

mode="${REGISTRYPULSE_MODE:-frontend}"

case "$mode" in
  api)
    exec /api "$@"
    ;;
  worker)
    exec /worker "$@"
    ;;
  frontend)
    config=/etc/nginx/registrypulse/frontend.conf
    ;;
  edge)
    config=/etc/nginx/registrypulse/edge.conf
    ;;
  *)
    echo "unsupported REGISTRYPULSE_MODE: $mode" >&2
    exit 64
    ;;
esac

rm -f /etc/nginx/conf.d/default.conf
cp "$config" /etc/nginx/conf.d/default.conf

if [ "$#" -eq 0 ]; then
  set -- nginx -g 'daemon off;'
fi

exec /docker-entrypoint.sh "$@"
