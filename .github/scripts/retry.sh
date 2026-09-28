#!/usr/bin/env bash
# Retry a network-bound command with exponential backoff.
#
# Usage: retry.sh [-n attempts] [-d initial_delay_seconds] [--] command [args...]
#
# Defaults: 3 attempts, 10s initial delay (doubled after each failure).
# Exits with the command's last exit status once every attempt has failed,
# so a real failure still fails the step.
set -euo pipefail

attempts=3
delay=10

while getopts ":n:d:" opt; do
  case "$opt" in
    n) attempts="$OPTARG" ;;
    d) delay="$OPTARG" ;;
    *)
      echo "usage: $0 [-n attempts] [-d initial_delay_seconds] [--] command [args...]" >&2
      exit 2
      ;;
  esac
done
shift $((OPTIND - 1))

if [[ $# -eq 0 ]]; then
  echo "usage: $0 [-n attempts] [-d initial_delay_seconds] [--] command [args...]" >&2
  exit 2
fi

for ((attempt = 1; ; attempt++)); do
  status=0
  "$@" || status=$?
  if [[ $status -eq 0 ]]; then
    exit 0
  fi
  if ((attempt >= attempts)); then
    echo "::error::'$*' failed after ${attempt} attempts (exit ${status})"
    exit "$status"
  fi
  echo "::warning::'$*' failed (attempt ${attempt}/${attempts}, exit ${status}); retrying in ${delay}s"
  sleep "$delay"
  delay=$((delay * 2))
done
