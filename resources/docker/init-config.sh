#!/bin/bash
# Initialize /etc/nginx on first boot, and upgrade bundled project files
# (e.g. conf.d/nginx-ui.conf) when they are still byte-equal to a known
# historical official default. Customized files are left alone; the UI
# self_check task surfaces them as a one-click fix.
#
# Override paths via env vars (used by bats tests).
# Sourcing the script with `--testing` defines functions and returns without
# running init_config_main; any other invocation runs main against the caller's
# environment.
set -u  # explicit failure handling; -e would skip our recovery paths

: "${ETC_NGINX:=/etc/nginx}"
: "${TEMPLATE_DIR:=/usr/local/etc/nginx}"
: "${HASH_FILE:=/usr/local/share/nginx-ui/nginx-ui.conf.known-hashes}"

log() { echo "[$1] init-config: $2"; }

# sync_bundled_file <template> <target> <known_hashes_file>
sync_bundled_file() {
    local template="$1" target="$2" hashes="$3"

    [ -f "$template" ] || { log WARN "template missing: $template"; return 0; }
    [ -f "$hashes"   ] || { log WARN "hash list missing: $hashes"; return 0; }
    if [ ! -f "$target" ]; then
        log INFO "target absent; copying template to $target"
        cp -p "$template" "$target"
        return $?
    fi

    if ! command -v sha256sum >/dev/null 2>&1; then
        log WARN "sha256sum unavailable; skipping sync of $target"
        return 0
    fi

    local cur_hash tpl_hash
    cur_hash="$(sha256sum "$target"   | awk '{print $1}')"
    tpl_hash="$(sha256sum "$template" | awk '{print $1}')"

    if [ "$cur_hash" = "$tpl_hash" ]; then
        return 0   # already up-to-date
    fi

    if grep -vE '^[[:space:]]*(#|$)' "$hashes" | awk '{print $1}' \
         | grep -Fxq "$cur_hash"; then
        local bak="${target}.bak.$(date +%Y%m%d%H%M%S)"
        if ! cp -p "$target" "$bak"; then
            log ERROR "backup failed ($target -> $bak); leaving file untouched"
            return 1
        fi
        local tmp="${target}.tmp.$$"
        if ! cp -p "$template" "$tmp" || ! mv -f "$tmp" "$target"; then
            log ERROR "write failed; restoring from $bak"
            cp -p "$bak" "$target" 2>/dev/null
            rm -f "$tmp" 2>/dev/null
            return 1
        fi
        log INFO "Synced $target from bundled template (old saved as $bak)"
    else
        log INFO "Skipping $target: customized (hash $cur_hash). See UI self-check."
    fi
}

init_config_main() {
    # Early exit: host_via_ssh mode (must come first; see spec §11).
    if [ "${NGINX_UI_DISABLE_BUNDLED_NGINX:-}" = "true" ]; then
        log INFO "host mode: skipping bundled nginx config initialization"
        return 0
    fi

    # Fresh-install seed path (preserves prior behaviour).
    if [ "$(ls -A "$ETC_NGINX" 2>/dev/null)" = "" ]; then
        cp -rp "$TEMPLATE_DIR"/* "$ETC_NGINX/"
        log INFO "Nginx configurations directory initialized"
        return 0
    fi

    # User opt-out for the upgrade-existing path.
    if [ "${NGINX_UI_PRESERVE_BUNDLED_CONF:-}" = "true" ]; then
        log INFO "NGINX_UI_PRESERVE_BUNDLED_CONF=true; skipping bundled-conf sync"
        return 0
    fi

    # Whitelist of bundled files we own and may upgrade.
    sync_bundled_file \
        "$TEMPLATE_DIR/conf.d/nginx-ui.conf" \
        "$ETC_NGINX/conf.d/nginx-ui.conf" \
        "$HASH_FILE"
}

# Only run main when executed directly; --testing lets bats source us.
case "${1:-}" in
    --testing) return 0 ;;
    *) init_config_main ;;
esac
