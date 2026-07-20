#!/usr/bin/env bash
set -u

if [[ $# -ne 2 ]]; then
  printf 'usage: %s LOG_DIRECTORY NATS_CONTAINER\n' "$0" >&2
  exit 2
fi

logs=$1
nats_name=$2
mkdir -p "$logs"

record_first() {
  local target=$1
  shift
  local scratch="${target}.tmp.$$"
  "$@" >"$scratch" 2>&1 || true
  mv -n "$scratch" "$target" 2>/dev/null || true
  rm -f "$scratch"
}

if [[ -f "$logs/worker.pid" ]]; then
  worker_pid="$(cat "$logs/worker.pid")"
  if [[ "$worker_pid" =~ ^[0-9]+$ ]]; then
    kill "$worker_pid" >/dev/null 2>&1 || true
    for attempt in {1..20}; do
      if ! kill -0 "$worker_pid" 2>/dev/null; then break; fi
      sleep 0.1
    done
    kill -KILL "$worker_pid" >/dev/null 2>&1 || true
  fi
  rm -f "$logs/worker.pid"
fi

if command -v docker >/dev/null 2>&1; then
  record_first "$logs/final-nats.log" timeout 20s docker logs "$nats_name"
  record_first "$logs/final-rm.log" timeout 30s docker rm -f "$nats_name"
else
  record_first "$logs/final-nats.log" printf '%s\n' \
    'docker unavailable before cleanup'
fi
