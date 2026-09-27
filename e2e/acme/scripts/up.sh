#!/bin/sh
# Start a fresh ACME E2E stack: remove the previous one with its volumes and
# data directory, start the containers and wait until nginx-ui answers
# /api/install with its install secret written.
set -eu

# shellcheck source=SCRIPTDIR/env.sh
. "$(dirname -- "$0")/env.sh"

E2E_ACME_KEEP_DATA=0 sh "$ACME_DIR/scripts/down.sh"

# Holds the per-run admin credentials and browser state (see lib/env.ts).
mkdir -p "$E2E_ACME_DATA_DIR"
acme_log "project=$E2E_ACME_PROJECT ui=http://127.0.0.1:$E2E_ACME_UI_PORT subnet=$E2E_ACME_SUBNET_PREFIX.0/24 data=$E2E_ACME_DATA_DIR image=$NGINX_UI_IMAGE"

acme_compose up -d --quiet-pull

fail() {
    acme_log "$1"
    acme_compose ps -a || true
    acme_compose logs --tail 80 || true
    exit 1
}

deadline=$(( $(date +%s) + ${E2E_ACME_BOOT_TIMEOUT:-180} ))
until curl --fail --silent --max-time 5 "http://127.0.0.1:$E2E_ACME_UI_PORT/api/install" >/dev/null 2>&1 \
    && acme_compose exec -T nginx-ui test -s /etc/nginx-ui/.install_secret; do
    [ "$(date +%s)" -lt "$deadline" ] || fail "nginx-ui did not become ready in time"
    sleep 2
done

if [ -n "$(acme_compose ps -a -q --status exited --status dead)" ]; then
    fail "a service exited during startup"
fi

acme_log "stack is up"
