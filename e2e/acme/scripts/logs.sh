#!/bin/sh
# Dump the ACME E2E stack's container logs and the bundled Nginx logs into
# a directory (default: e2e/acme/test-results).
set -u

# shellcheck source=SCRIPTDIR/env.sh
. "$(dirname -- "$0")/env.sh"

out="${1:-$ACME_DIR/test-results}"
mkdir -p "$out"
acme_compose logs --no-color --timestamps > "$out/compose.log" 2>&1
acme_compose exec -T nginx-ui sh -c 'echo "== error.log"; tail -n 1000 /var/log/nginx/error.log; echo "== access.log"; tail -n 1000 /var/log/nginx/access.log' \
    > "$out/nginx.log" 2>&1
acme_log "wrote $out/compose.log and $out/nginx.log"
