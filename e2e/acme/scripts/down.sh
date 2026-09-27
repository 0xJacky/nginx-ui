#!/bin/sh
# Stop the ACME E2E stack, remove its volumes and its data directory.
# E2E_ACME_KEEP_DATA=1 keeps the containers' volumes and the data directory
# (credentials) for debugging.
set -eu

# shellcheck source=SCRIPTDIR/env.sh
. "$(dirname -- "$0")/env.sh"

if [ "${E2E_ACME_KEEP_DATA:-0}" = "1" ]; then
    acme_compose down --remove-orphans --timeout 5 >/dev/null 2>&1 || true
    acme_log "keeping volumes and $E2E_ACME_DATA_DIR"
    exit 0
fi

acme_compose down --volumes --remove-orphans --timeout 5 >/dev/null 2>&1 || true

case "$E2E_ACME_DATA_DIR" in
    ""|"/"|"$HOME"|"$REPO_ROOT")
        acme_log "refusing to remove E2E_ACME_DATA_DIR=$E2E_ACME_DATA_DIR"
        exit 1
        ;;
esac

if [ -d "$E2E_ACME_DATA_DIR" ]; then
    rm -rf "$E2E_ACME_DATA_DIR"
    acme_log "removed $E2E_ACME_DATA_DIR"
fi
