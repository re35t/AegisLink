#!/usr/bin/env bash

set -Eeuo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
dev_pid_file="$repo_root/.aegislink-dev/dev.pid"

is_dev_supervisor() {
  local pid=$1
  local process_cwd process_argument

  [[ "$pid" =~ ^[0-9]+$ ]] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  process_cwd="$(readlink -f "/proc/$pid/cwd" 2>/dev/null || true)"
  [[ "$process_cwd" == "$repo_root" ]] || return 1

  while IFS= read -r -d '' process_argument; do
    case "$process_argument" in
      scripts/dev.sh | ./scripts/dev.sh | "$repo_root/scripts/dev.sh")
        return 0
        ;;
    esac
  done <"/proc/$pid/cmdline"

  return 1
}

find_dev_supervisor() {
  local pid

  if [[ -s "$dev_pid_file" ]]; then
    pid="$(<"$dev_pid_file")"
    if is_dev_supervisor "$pid"; then
      printf '%s\n' "$pid"
      return 0
    fi
    rm -f "$dev_pid_file"
  fi

  return 1
}

if ! supervisor_pid="$(find_dev_supervisor)"; then
  printf 'AegisLink development stack is not running.\n'
  exit 0
fi

printf 'Stopping AegisLink development stack (PID %s)...\n' "$supervisor_pid"
kill -TERM "$supervisor_pid"

for _ in {1..100}; do
  if ! kill -0 "$supervisor_pid" 2>/dev/null; then
    rm -f "$dev_pid_file"
    printf 'AegisLink development stack stopped.\n'
    exit 0
  fi
  if [[ -r "/proc/$supervisor_pid/stat" ]] && [[ "$(cut -d ' ' -f 3 "/proc/$supervisor_pid/stat")" == "Z" ]]; then
    rm -f "$dev_pid_file"
    printf 'AegisLink development stack stopped.\n'
    exit 0
  fi
  sleep 0.1
done

printf 'Timed out waiting for PID %s to stop.\n' "$supervisor_pid" >&2
exit 1
