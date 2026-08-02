#!/bin/sh
# Deploy the demo Worker against an already-published container image.
#
# The default wrangler.jsonc builds the image from ../demo.Dockerfile, which is
# what you want locally. In CI the image has already been built and pushed by
# the docker-build job, so rebuilding it would burn another six-platform QEMU
# run and push the same bytes twice. This swaps the `image` field for the
# published reference and deploys, leaving wrangler.jsonc untouched.
#
# Usage: sh deploy-published.sh docker.io/uozi/nginx-ui-demo:sha-abc1234
#
# Pass an immutable tag, not :latest. Cloudflare pins the container application
# to a specific image, so redeploying with an unchanged reference does not roll
# the running instance onto new bytes.
set -eu

if [ $# -ne 1 ]; then
    echo "usage: $0 <image-reference>" >&2
    exit 2
fi

image="$1"
here="$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)"
cd "$here"

case "$image" in
    *:latest)
        echo "refusing to deploy a mutable :latest tag — pass an immutable one" >&2
        exit 2
        ;;
esac

# Must live beside wrangler.jsonc, not in a temp directory: wrangler resolves
# `main` and the container build context relative to the config file, so a
# config in /tmp sends it looking for /tmp/src/index.ts.
generated="wrangler.published.jsonc"
trap 'rm -f "$generated"' EXIT INT TERM

# Replace the local-build block with the published reference. Done with a
# targeted substitution rather than a JSON rewrite because wrangler.jsonc
# carries comments that a JSON parser would discard.
python3 - "$image" "$generated" <<'PY'
import re
import sys

image, out = sys.argv[1], sys.argv[2]
source = open('wrangler.jsonc', encoding='utf-8').read()

pattern = re.compile(
    r'"image":\s*"\.\./demo\.Dockerfile",\s*'
    r'"image_build_context":\s*"[^"]*",\s*'
    r'"image_vars":\s*\{[^}]*\},',
    re.S,
)

patched, count = pattern.subn(f'"image": "{image}",', source)
if count != 1:
    raise SystemExit(
        f'expected exactly one local-build image block in wrangler.jsonc, found {count}'
    )

open(out, 'w', encoding='utf-8').write(patched)
PY

echo "deploying with image $image"
bunx wrangler deploy -c "$generated"
