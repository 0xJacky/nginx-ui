#!/bin/sh
# Build the official nginx-ui image (Dockerfile, not demo.Dockerfile) from the
# current working tree for the ACME E2E stack.
#
#   E2E_ACME_BUILD_FRONTEND=1   rebuild app/dist even when it exists
#   E2E_ACME_BINARY=native      go build on this host (Linux amd64 + CGO toolchain)
#   E2E_ACME_BINARY=docker      cross-build in golang:*-trixie via cloudflare/build-binary.sh
#   E2E_ACME_BINARY=skip        reuse nginx-ui-linux-amd64/nginx-ui as is
# The default is native on Linux x86_64 with Go installed, docker elsewhere.
set -eu

# shellcheck source=SCRIPTDIR/env.sh
. "$(dirname -- "$0")/env.sh"
cd "$REPO_ROOT"

if [ "${E2E_ACME_BUILD_FRONTEND:-0}" = "1" ] || [ ! -f app/dist/index.html ]; then
    acme_log "building the frontend (app/dist)"
    [ -d node_modules ] || bun install --frozen-lockfile
    bun run build
fi

binary_mode="${E2E_ACME_BINARY:-}"
if [ -z "$binary_mode" ]; then
    if [ "$(uname -s)" = "Linux" ] && [ "$(uname -m)" = "x86_64" ] && command -v go >/dev/null 2>&1; then
        binary_mode=native
    else
        binary_mode=docker
    fi
fi

case "$binary_mode" in
    native)
        acme_log "building nginx-ui-linux-amd64/nginx-ui natively"
        # Same flags as the release build in .github/workflows/build.yml.
        GOWORK=off CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -tags=jsoniter \
            -ldflags "-s -w -X 'github.com/0xJacky/Nginx-UI/settings.buildTime=$(date +%s)'" \
            -o nginx-ui-linux-amd64/nginx-ui main.go
        ;;
    docker)
        acme_log "cross-building nginx-ui-linux-amd64/nginx-ui in a container"
        sh cloudflare/build-binary.sh
        ;;
    skip)
        [ -x nginx-ui-linux-amd64/nginx-ui ] || { acme_log "nginx-ui-linux-amd64/nginx-ui is missing"; exit 1; }
        acme_log "reusing nginx-ui-linux-amd64/nginx-ui"
        ;;
    *)
        acme_log "unknown E2E_ACME_BINARY=$binary_mode (native|docker|skip)"
        exit 1
        ;;
esac

acme_log "building image $NGINX_UI_IMAGE"
docker build -f Dockerfile --platform linux/amd64 -t "$NGINX_UI_IMAGE" \
    --build-arg TARGETOS=linux --build-arg TARGETARCH=amd64 .
