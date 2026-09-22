#!/bin/sh
# Copies the generated contract package from the nginx-ui-plugin-spec
# repository, which is the source of truth. The files are verbatim copies.
#
# Usage: ./regen.sh [--check]
#   --check  only report files that differ from the spec repository
# SPEC_DIR overrides the path of the nginx-ui-plugin-spec checkout.
set -eu

here=$(cd "$(dirname "$0")" && pwd)
spec=${SPEC_DIR:-$here/../../../../../nginx-ui-plugin-spec}
if ! src=$(cd "$spec/gen/go/nginxui/plugin/v1" 2>/dev/null && pwd); then
	echo "regen.sh: $spec/gen/go/nginxui/plugin/v1 not found, set SPEC_DIR" >&2
	exit 1
fi

if [ "${1:-}" = "--check" ]; then
	status=0
	for f in "$src"/*.pb.go; do
		name=$(basename "$f")
		if ! cmp -s "$f" "$here/$name"; then
			echo "stale: $name" >&2
			status=1
		fi
	done
	for f in "$here"/*.pb.go; do
		[ -e "$f" ] || continue
		name=$(basename "$f")
		if [ ! -e "$src/$name" ]; then
			echo "extra: $name" >&2
			status=1
		fi
	done
	exit "$status"
fi

rm -f "$here"/*.pb.go
cp "$src"/*.pb.go "$here"/
echo "copied $(ls "$src"/*.pb.go | wc -l | tr -d ' ') files from $src"
