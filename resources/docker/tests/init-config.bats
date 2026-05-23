#!/usr/bin/env bats

setup() {
    TMP="$(mktemp -d)"
    export ETC_NGINX="$TMP/etc-nginx"
    export TEMPLATE_DIR="$TMP/usr-local/etc/nginx"
    export HASH_FILE="$TMP/known-hashes"
    mkdir -p "$ETC_NGINX/conf.d" "$TEMPLATE_DIR/conf.d"

    cp "$BATS_TEST_DIRNAME/fixtures/fixed-default.conf" \
       "$TEMPLATE_DIR/conf.d/nginx-ui.conf"
    # Hash list = current template hash + historical (unfixed) hash.
    sha256sum "$TEMPLATE_DIR/conf.d/nginx-ui.conf" | awk '{print $1}'  > "$HASH_FILE"
    sha256sum "$BATS_TEST_DIRNAME/fixtures/unfixed-default.conf" | awk '{print $1}' >> "$HASH_FILE"

    unset NGINX_UI_DISABLE_BUNDLED_NGINX NGINX_UI_PRESERVE_BUNDLED_CONF

    # Source the script in test mode so functions are defined but main is not run.
    # shellcheck disable=SC1091
    source "$BATS_TEST_DIRNAME/../init-config.sh" --testing
}

teardown() { rm -rf "$TMP"; }

@test "fresh empty dir copies entire template" {
    rm -rf "$ETC_NGINX"/*
    run init_config_main
    [ "$status" -eq 0 ]
    [ -f "$ETC_NGINX/conf.d/nginx-ui.conf" ]
}

@test "current-template hash is no-op (no backup created)" {
    cp "$TEMPLATE_DIR/conf.d/nginx-ui.conf" "$ETC_NGINX/conf.d/nginx-ui.conf"
    run init_config_main
    [ "$status" -eq 0 ]
    bak_count=$(ls "$ETC_NGINX/conf.d/"*.bak.* 2>/dev/null | wc -l)
    [ "$bak_count" -eq 0 ]
}

@test "historical hash upgrades the file with a timestamped backup" {
    cp "$BATS_TEST_DIRNAME/fixtures/unfixed-default.conf" \
       "$ETC_NGINX/conf.d/nginx-ui.conf"
    run init_config_main
    [ "$status" -eq 0 ]
    bak=$(ls "$ETC_NGINX/conf.d/"nginx-ui.conf.bak.* 2>/dev/null | head -n1)
    [ -n "$bak" ]
    diff -q "$ETC_NGINX/conf.d/nginx-ui.conf" "$TEMPLATE_DIR/conf.d/nginx-ui.conf"
    diff -q "$bak" "$BATS_TEST_DIRNAME/fixtures/unfixed-default.conf"
}

@test "unknown hash (customized) skips file and logs a message" {
    printf '# user-customized stub\n' > "$ETC_NGINX/conf.d/nginx-ui.conf"
    before=$(cat "$ETC_NGINX/conf.d/nginx-ui.conf")
    run init_config_main
    [ "$status" -eq 0 ]
    [ "$(cat "$ETC_NGINX/conf.d/nginx-ui.conf")" = "$before" ]
    [[ "$output" == *"customized"* ]]
    bak_count=$(ls "$ETC_NGINX/conf.d/"*.bak.* 2>/dev/null | wc -l)
    [ "$bak_count" -eq 0 ]
}

@test "NGINX_UI_PRESERVE_BUNDLED_CONF=true never syncs" {
    export NGINX_UI_PRESERVE_BUNDLED_CONF=true
    cp "$BATS_TEST_DIRNAME/fixtures/unfixed-default.conf" \
       "$ETC_NGINX/conf.d/nginx-ui.conf"
    run init_config_main
    [ "$status" -eq 0 ]
    bak_count=$(ls "$ETC_NGINX/conf.d/"*.bak.* 2>/dev/null | wc -l)
    [ "$bak_count" -eq 0 ]
    diff -q "$ETC_NGINX/conf.d/nginx-ui.conf" \
            "$BATS_TEST_DIRNAME/fixtures/unfixed-default.conf"
}

@test "NGINX_UI_DISABLE_BUNDLED_NGINX=true wins over PRESERVE" {
    export NGINX_UI_DISABLE_BUNDLED_NGINX=true
    export NGINX_UI_PRESERVE_BUNDLED_CONF=true
    run init_config_main
    [ "$status" -eq 0 ]
    [[ "$output" == *"host mode"* ]]
}

@test "backup failure does not overwrite target" {
    [ "$(id -u)" -ne 0 ] || skip "chmod-based perm test is no-op as root"
    cp "$BATS_TEST_DIRNAME/fixtures/unfixed-default.conf" \
       "$ETC_NGINX/conf.d/nginx-ui.conf"
    chmod 555 "$ETC_NGINX/conf.d"
    run init_config_main
    chmod 755 "$ETC_NGINX/conf.d"
    [ "$status" -eq 0 ]
    [[ "$output" == *"backup failed"* ]]
    diff -q "$ETC_NGINX/conf.d/nginx-ui.conf" \
            "$BATS_TEST_DIRNAME/fixtures/unfixed-default.conf"
}
