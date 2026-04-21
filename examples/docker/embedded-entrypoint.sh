#!/usr/bin/env sh
set -eu

reverse-client daemon &
client_pid=$!

cleanup() {
  kill "$client_pid" 2>/dev/null || true
}
trap cleanup INT TERM EXIT

exec "$@"
