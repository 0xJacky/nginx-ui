#!/bin/sh
# Produce the linux/amd64 nginx-ui binary that demo.Dockerfile copies in.
#
# Cross-compiling from macOS needs a CGO toolchain (the SQLite driver), so this
# builds inside a Linux container rather than on the host. The frontend must be
# built first: app/dist is embedded into the binary at compile time.
set -eu

repo_root="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$repo_root"

if [ ! -f app/dist/index.html ]; then
    echo "app/dist is missing; building the frontend first" >&2
    bun run build
fi

echo "building nginx-ui for linux/amd64 (this takes a few minutes on a cold cache)"
# Same flags as the release build in .github/workflows/build.yml, plus -s.
# An unstripped binary is ~166 MB, which dominates the image and therefore the
# cold start, since Cloudflare distributes the image before an instance runs.
docker run --rm --platform linux/amd64 \
    -v "$repo_root":/src -w /src \
    -v "$HOME/go/pkg/mod":/go/pkg/mod \
    -e GOWORK=off -e CGO_ENABLED=1 -e GOOS=linux -e GOARCH=amd64 \
    golang:1.26-trixie \
    go build -trimpath -tags=jsoniter \
        -ldflags "-s -w -X 'github.com/0xJacky/Nginx-UI/settings.buildTime=$(date +%s)'" \
        -o /src/nginx-ui-linux-amd64/nginx-ui main.go

ls -la nginx-ui-linux-amd64/nginx-ui
