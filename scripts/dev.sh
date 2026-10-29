#!/usr/bin/env bash
# Go API on :8080 and Vite on :5173. Ctrl+C, or either one exiting, stops both.
set -uo pipefail

./build/server &
go_pid=$!

(cd web && exec node_modules/.bin/vite) &
web_pid=$!

stop() {
  kill "$go_pid" "$web_pid" 2>/dev/null
  wait 2>/dev/null
}
trap stop INT TERM EXIT

wait -n
