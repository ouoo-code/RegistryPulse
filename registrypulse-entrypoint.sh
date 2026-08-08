#!/bin/sh
set -eu

mode="${REGISTRYPULSE_MODE:-frontend}"
api_port="${API_HTTP_PORT:-8080}"

case "$api_port" in
  ''|*[!0-9]*)
    echo "invalid API_HTTP_PORT: $api_port" >&2
    exit 64
    ;;
esac

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
if [ "$mode" = "edge" ]; then
  sed "s/__API_HTTP_PORT__/$api_port/g" "$config" > /etc/nginx/conf.d/default.conf
else
  cp "$config" /etc/nginx/conf.d/default.conf
fi

if [ "$#" -eq 0 ]; then
  set -- nginx -g 'daemon off;'
fi

exec /docker-entrypoint.sh "$@"
