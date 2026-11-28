#!/usr/bin/env bash
# Fail when bun.lock resolves more than one copy of @v-c/util.
#
# Usage: check-single-v-c-util.sh [--fix] [lockfile]
#
# @v-c/util/dist/Dom/focus.js keeps its focus-lock stack in module state.
# When @v-c/dialog and @v-c/drawer load different copies, a modal opened over
# a drawer fights the drawer for focus (the 2FA OTP modal over the
# access-list drawer stays stuck on its first box). bun never re-resolves an
# existing hoisted entry, so dependency bumps that need a newer util add
# nested "<pkg>/@v-c/util" copies instead of moving the root one.
#
# --fix rewrites the lockfile so the root entry is the highest version already
# in it and drops every nested entry. Run `bun install` with the CI-pinned bun
# afterwards, then run this check again: if some package needs a version the
# lockfile does not hold yet, bun nests it again and the check still fails.
set -euo pipefail

fix=false
if [ "${1:-}" = "--fix" ]; then
  fix=true
  shift
fi
lockfile="${1:-bun.lock}"
pkg='@v-c/util'
pkg_re='@v-c\/util'

root_re="^    \"${pkg_re}\": \\["
nested_re="^    \"[^\"]+\\/${pkg_re}\": \\["

nested=$(grep -E "$nested_re" "$lockfile" || true)
if [ -z "$nested" ]; then
  root_count=$(grep -cE "$root_re" "$lockfile" || true)
  echo "OK: ${lockfile} resolves ${pkg} once (${root_count} root entry)."
  exit 0
fi

# "<key> <name@version>" for every entry, root included.
entries=$(grep -E "^    \"([^\"]+\\/)?${pkg_re}\": \\[" "$lockfile" |
  sed -E 's/^    "([^"]+)": \["([^"]+)".*/\1 \2/')

if [ "$fix" = false ]; then
  echo "::error file=${lockfile}::${lockfile} resolves more than one copy of ${pkg}."
  echo "Overlays on different copies fight over focus (modal over drawer)."
  echo "Entries:"
  printf '  %s\n' "${entries//$'\n'/$'\n'  }"
  echo
  echo "Fix: $0 --fix ${lockfile} && bun install (CI-pinned bun), then rerun this check."
  exit 1
fi

highest=$(echo "$entries" | awk '{print $2}' | sed "s|^${pkg}@||" | sort -uV | tail -1)
majors=$(echo "$entries" | awk '{print $2}' | sed "s|^${pkg}@||" | cut -d. -f1 | sort -u | wc -l | tr -d ' ')
if [ "$majors" != 1 ]; then
  echo "Refusing to fix: ${lockfile} mixes major versions of ${pkg}; resolve it by hand." >&2
  exit 1
fi

if ! grep -qE "$root_re" "$lockfile"; then
  echo "Refusing to fix: ${lockfile} has no root ${pkg} entry to replace." >&2
  exit 1
fi

source_line=$(grep -E "^    \"([^\"]+\\/)?${pkg_re}\": \\[\"${pkg_re}@${highest}\"" "$lockfile" | head -1)
root_line=$(echo "$source_line" | sed -E "s#^    \"[^\"]*\"#    \"${pkg}\"#")

# Pass the patterns through the environment: awk -v would eat their backslashes.
tmp=$(mktemp)
REPL="$root_line" ROOT_RE="$root_re" NESTED_RE="$nested_re" awk '
  $0 ~ ENVIRON["NESTED_RE"] { next }
  $0 ~ ENVIRON["ROOT_RE"] { print ENVIRON["REPL"]; next }
  { print }
' "$lockfile" > "$tmp"
cat "$tmp" > "$lockfile"
rm -f "$tmp"

echo "Rewrote ${lockfile}: root ${pkg}@${highest}, nested entries removed."
echo "Now run bun install with the CI-pinned bun and rerun this check."
