#!/usr/bin/env bash

set -Eeuo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

declare -a service_pids=()
agent_key_override="${AGENT_KEY_ENCRYPTION_KEY:-}"

load_env_file() {
  local path=$1
  if [[ -f "$path" ]]; then
    set -a
    # Development env files use shell-compatible KEY=value syntax.
    source "$path"
    set +a
  fi
}

load_env_file "$repo_root/.env"
load_env_file "$repo_root/.env.local"
if [[ -n "$agent_key_override" ]]; then
  AGENT_KEY_ENCRYPTION_KEY="$agent_key_override"
  export AGENT_KEY_ENCRYPTION_KEY
fi

cleanup() {
  local status=$?
  trap - EXIT INT TERM

  if ((${#service_pids[@]} > 0)); then
    printf '\nStopping local development services...\n'
    local pid
    for pid in "${service_pids[@]}"; do
      if kill -0 "$pid" 2>/dev/null; then
        kill -TERM -- "-$pid" 2>/dev/null || true
      fi
    done
    for pid in "${service_pids[@]}"; do
      wait "$pid" 2>/dev/null || true
    done
  fi

  printf 'Development PostgreSQL containers remain running; data volumes are preserved.\n'
  exit "$status"
}

trap 'exit 130' INT
trap 'exit 143' TERM
trap cleanup EXIT

for command_name in docker go openssl pnpm setsid; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    printf 'Required command not found: %s\n' "$command_name" >&2
    exit 1
  fi
done

if [[ -z "${AGENT_KEY_ENCRYPTION_KEY:-}" ]]; then
  dev_state_dir="$repo_root/.aegislink-dev"
  dev_agent_key_file="$dev_state_dir/agent-key"
  umask 077
  mkdir -p "$dev_state_dir"
  if [[ ! -s "$dev_agent_key_file" ]]; then
    openssl rand -base64 32 >"$dev_agent_key_file"
    printf 'Generated a stable local Agent encryption key in .aegislink-dev/agent-key.\n'
  fi
  AGENT_KEY_ENCRYPTION_KEY="$(tr -d '\r\n' <"$dev_agent_key_file")"
  export AGENT_KEY_ENCRYPTION_KEY
fi

printf 'Starting development PostgreSQL containers...\n'
docker compose up -d --wait postgres index-postgres

start_service() {
  local name=$1
  shift
  printf 'Starting %s...\n' "$name"
  setsid "$@" &
  service_pids+=("$!")
}

start_service "Agent Server" go run ./cmd/aegislink-server
start_service "AegisLink Index" go run ./index/cmd/aegislink-index
start_service "Vite Web" pnpm dev:web

printf '\nAegisLink development stack is running:\n'
printf '  Web:    http://127.0.0.1:5173\n'
printf '  API:    http://127.0.0.1:4321\n'
printf '  Index:  http://127.0.0.1:4331\n'
printf 'Press Ctrl-C to stop the three local services.\n\n'

set +e
wait -n "${service_pids[@]}"
status=$?
set -e

if ((status == 0)); then
  printf '\nA development service exited; stopping the remaining services.\n'
else
  printf '\nA development service failed with status %d; stopping the remaining services.\n' "$status" >&2
fi
exit "$status"
