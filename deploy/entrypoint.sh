#!/bin/sh
set -eu

if [ "${1:-}" = initialize ]; then
  exec /usr/bin/tini -- /bin/sh /app/deploy/initialize.sh
fi

if [ -n "${TSW_DATABASE_URL_FILE:-}" ]; then
  if [ -n "${TSW_DATABASE_URL:-}" ]; then
    printf '%s\n' 'Set only one of TSW_DATABASE_URL and TSW_DATABASE_URL_FILE.' >&2
    exit 1
  fi
  TSW_DATABASE_URL=$(cat "$TSW_DATABASE_URL_FILE")
  export TSW_DATABASE_URL
fi

case "${1:-}" in
  control|gateway)
    exec /usr/bin/tini -g -- xvfb-run -a /apophis-teamseatwatch "$@"
    ;;
  *)
    exec /usr/bin/tini -- /apophis-teamseatwatch "$@"
    ;;
esac
