#!/bin/sh
# Demo container entrypoint.
#
# The production image uses s6-overlay, which cannot run here: s6's preinit
# chowns /run and its suexec calls setgid, and Cloudflare Containers grant
# neither CAP_CHOWN nor CAP_SETGID, so s6 exits 111 before anything starts.
#
# Note also that "runs as root" does not mean unrestricted. Every capability is
# dropped, so without CAP_DAC_OVERRIDE root obeys ordinary file permissions and
# nothing in the image may be owned by another user.
set -eu

: "${PEER_COUNT:=2}"
BOOT_LOG=/var/log/nginx/demo-boot.log

log() { echo "[entrypoint] $*" | tee -a "$BOOT_LOG"; }

mkdir -p /var/log/nginx
: > "$BOOT_LOG"

# Seed /etc/nginx on first boot, mirroring resources/docker/init-config.sh but
# without the s6 oneshot wrapper.
if [ -z "$(ls -A /etc/nginx 2>/dev/null)" ]; then
    cp -rp /usr/local/etc/nginx/* /etc/nginx/
    log "initialized /etc/nginx from the bundled template"
fi

# nginx creates its temp dirs lazily; make them now so a request in the first
# second does not race directory creation.
mkdir -p /tmp/nginx-client-body /tmp/nginx-proxy /tmp/nginx-fastcgi \
         /tmp/nginx-uwsgi /tmp/nginx-scgi

# The working directories must be created at RUNTIME, not in the image.
# /run is a fresh tmpfs on Cloudflare Containers, so anything mkdir'd there at
# build time is gone by the time this script runs, and nginx-ui fails with
# "bind: no such file or directory" on its risefront handover socket.
mkdir -p /var/run/nginx-ui
n=2
while [ "$n" -le $((PEER_COUNT + 1)) ]; do
    mkdir -p "/var/run/nginx-ui-node${n}"
    n=$((n + 1))
done

# nginx comes up first and stays up for the container's whole life. Keeping it
# alive even when nginx-ui is down is deliberate: the platform health check and
# the boot log below stay reachable, so a failure is diagnosable instead of
# looking like a container that simply never started.
nginx -g "daemon off;" >> "$BOOT_LOG" 2>&1 &
NGINX_PID=$!
log "started nginx (pid $NGINX_PID)"

# Every nginx-ui this script started. Recorded because the shutdown handler has
# to signal them by PID: this image is built on nginx:latest, which ships no
# procps, so `pkill` is not a command here — it exits 127.
UI_PIDS=""

start_ui() {
    name="$1"
    workdir="$2"
    config="$3"
    NGINX_UI_WORKING_DIR="$workdir" nginx-ui --config "$config" >> "$BOOT_LOG" 2>&1 &
    UI_PIDS="$UI_PIDS $!"
    log "started $name (pid $!)"
}

# Peer nodes for the cluster view. Each needs its own working directory:
# risefront derives its handover socket path from it and would otherwise treat
# a peer as a hot-reload child of the primary.
n=2
while [ "$n" -le $((PEER_COUNT + 1)) ]; do
    conf="/etc/nginx-ui-node${n}/app.ini"
    if [ -f "$conf" ]; then
        start_ui "nginx-ui-node${n}" "/var/run/nginx-ui-node${n}" "$conf"
    fi
    n=$((n + 1))
done

start_ui nginx-ui "${NGINX_UI_WORKING_DIR:-/var/run/nginx-ui}" /etc/nginx-ui/app.ini

# Seconds each nginx-ui gets to close its SQLite handle before it is killed.
SHUTDOWN_GRACE=10

shutting_down=0

# Shut down in order and then actually exit.
#
# This runs on every stop the platform issues: the sleepAfter idle expiry, the
# daily restore, and a manual recycle. It previously called `pkill -TERM
# nginx-ui`, which does not exist in this image and so exited 127 into
# /dev/null — the nginx-ui children were never signalled, the bare `wait` below
# blocked on them forever, and `exit 0` was never reached. nginx was already
# dead by then, so the container sat there as PID 1 with nothing bound to 8080,
# reported healthy by the platform, for as long as anyone left it. Signalling
# recorded PIDs and bounding the wait is what keeps that from recurring.
shutdown() {
    if [ "$shutting_down" = 1 ]; then
        return
    fi
    shutting_down=1

    log "received termination signal"
    kill -TERM "$NGINX_PID" 2>/dev/null || true

    for pid in $UI_PIDS; do
        kill -TERM "$pid" 2>/dev/null || true
    done

    # nginx-ui holds an open SQLite handle, so give every child a chance to
    # close it — but never more than that. A child that will not exit must not
    # be able to keep the container alive.
    waited=0
    while [ "$waited" -lt "$SHUTDOWN_GRACE" ]; do
        alive=""
        for pid in $UI_PIDS; do
            if kill -0 "$pid" 2>/dev/null; then
                alive="$alive $pid"
            fi
        done
        if [ -z "$alive" ]; then
            break
        fi
        sleep 1
        waited=$((waited + 1))
    done

    for pid in $UI_PIDS; do
        if kill -0 "$pid" 2>/dev/null; then
            log "nginx-ui $pid did not exit in ${SHUTDOWN_GRACE}s; killing it"
            kill -KILL "$pid" 2>/dev/null || true
        fi
    done

    log "shutdown complete"
    exit 0
}
trap shutdown TERM INT

# Supervise nginx. The UI's "restart nginx" runs `nginx -s stop` (see
# RestartCmd in resources/demo/app.ini), so nginx exiting is a normal event
# that must bring it straight back rather than end the container.
#
# A crashed nginx-ui deliberately does NOT take the container down: it keeps
# serving 502 plus the boot log at /__demo/bootlog, and an opaque restart loop
# is far harder to debug than a 502.
restarts=0
while true; do
    if kill -0 "$NGINX_PID" 2>/dev/null; then
        sleep 5
        continue
    fi

    restarts=$((restarts + 1))
    if [ "$restarts" -gt 20 ]; then
        log "nginx exited $restarts times; giving up so the platform recycles the container"
        exit 1
    fi

    log "nginx exited; restarting it (attempt $restarts)"
    sleep 1
    nginx -g "daemon off;" >> "$BOOT_LOG" 2>&1 &
    NGINX_PID=$!
done
