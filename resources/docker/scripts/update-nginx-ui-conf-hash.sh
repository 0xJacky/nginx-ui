#!/bin/bash
# Append current SHA256 of resources/docker/nginx-ui.conf to the
# known-hashes file. Run this after editing the template, then commit
# both files together.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

template=resources/docker/nginx-ui.conf
list=resources/docker/nginx-ui.conf.known-hashes

[ -f "$template" ] || { echo "template not found: $template" >&2; exit 1; }
[ -f "$list"     ] || { echo "hash list not found: $list" >&2; exit 1; }

hash=$(sha256sum "$template" | awk '{print $1}')
last=$(grep -vE '^[[:space:]]*(#|$)' "$list" | awk '{print $1}' | tail -n1)

if [ "$hash" = "$last" ]; then
    echo "No change: hash $hash already at tail of $list"
    exit 0
fi

ver=$(git describe --tags --abbrev=0 2>/dev/null || echo "dev")
printf '%s  # %s (%s)\n' "$hash" "$ver" "$(date +%Y-%m-%d)" >> "$list"
echo "Appended: $hash  # $ver"
