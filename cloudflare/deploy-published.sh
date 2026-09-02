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

here="$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)"
cd "$here"

if [ $# -gt 1 ]; then
    echo "usage: $0 [image-reference]" >&2
    echo "  with no argument, resolves the image built for the current commit" >&2
    exit 2
fi

if [ $# -eq 1 ]; then
    image="$1"
else
    # Default to this commit's image rather than making the caller paste a SHA.
    # Pasting one is how a stale image once got redeployed over a newer one.
    commit="$(git rev-parse HEAD)"
    image="docker.io/uozi/nginx-ui-demo:sha-${commit}"
    echo "resolved image for HEAD (${commit%"${commit#?????????}"}): $image"
fi

case "$image" in
    *:latest)
        echo "refusing to deploy a mutable :latest tag — pass an immutable one" >&2
        exit 2
        ;;
esac

# Check the image exists before deploying. Not every commit produces one: the
# build workflow is path-filtered, so a Worker-only change has no image of its
# own and must reuse the previous commit's.
#
# Only an explicit "not found" is fatal. Reaching Docker Hub fails
# intermittently (transport EOF), and treating that as a missing image would
# block deploys for a network blip — so on any other error say so and let
# Cloudflare be the authority, since it reports a genuinely missing image
# clearly enough.
probe=""
for attempt in 1 2 3; do
    if probe="$(docker manifest inspect "$image" 2>&1)"; then
        probe=""
        break
    fi
    case "$probe" in
        *"manifest unknown"*|*"not found"*|*"no such manifest"*)
            echo "no published image at $image" >&2
            echo "the build workflow is path-filtered, so this commit may not have produced" >&2
            echo "an image. Pass the tag of the last built commit instead." >&2
            exit 1
            ;;
    esac
    [ "$attempt" -lt 3 ] && sleep 3
done

if [ -n "$probe" ]; then
    echo "warning: could not verify the image exists ($probe)" >&2
    echo "continuing; Cloudflare will reject a genuinely missing image" >&2
fi

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
