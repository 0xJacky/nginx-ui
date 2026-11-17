#!/usr/bin/env bats

setup() {
    TMP="$(mktemp -d)"
    FAKE_BIN="$TMP/bin"
    COMMAND_LOG="$TMP/commands.log"
    mkdir -p "$FAKE_BIN"

    cat > "$FAKE_BIN/nginx" <<'EOF'
#!/bin/sh
printf 'nginx:%s\n' "$*" >> "$COMMAND_LOG"
EOF
    cat > "$FAKE_BIN/sleep" <<'EOF'
#!/bin/sh
printf 'sleep:%s\n' "$*" >> "$COMMAND_LOG"
EOF
    chmod +x "$FAKE_BIN/nginx" "$FAKE_BIN/sleep"
}

teardown() { rm -rf "$TMP"; }

@test "disabled bundled Nginx keeps the service idle" {
    run env PATH="$FAKE_BIN:$PATH" \
        COMMAND_LOG="$COMMAND_LOG" \
        NGINX_UI_DISABLE_BUNDLED_NGINX=true \
        TZ= \
        sh "$BATS_TEST_DIRNAME/../nginx.run"

    [ "$status" -eq 0 ]
    grep -Fxq 'sleep:infinity' "$COMMAND_LOG"
    ! grep -q '^nginx:' "$COMMAND_LOG"
}

@test "enabled bundled Nginx starts in the foreground" {
    run env PATH="$FAKE_BIN:$PATH" \
        COMMAND_LOG="$COMMAND_LOG" \
        NGINX_UI_DISABLE_BUNDLED_NGINX=false \
        TZ= \
        sh "$BATS_TEST_DIRNAME/../nginx.run"

    [ "$status" -eq 0 ]
    grep -Fxq 'nginx:-g daemon off;' "$COMMAND_LOG"
    ! grep -q '^sleep:' "$COMMAND_LOG"
}

@test "enabled bundled Nginx replaces the service process" {
    # s6 supervises the PID it started, so Nginx must take it over through
    # exec rather than run as a child of the run script.
    cat > "$FAKE_BIN/nginx" <<'EOF'
#!/bin/sh
printf '%s\n' "$$" > "$NGINX_PID_LOG"
EOF
    chmod +x "$FAKE_BIN/nginx"

    run env PATH="$FAKE_BIN:$PATH" \
        NGINX_PID_LOG="$TMP/nginx.pid" \
        NGINX_UI_DISABLE_BUNDLED_NGINX=false \
        TZ= \
        sh -c 'echo "$$" > "$0/service.pid"; exec sh "$1"' "$TMP" "$BATS_TEST_DIRNAME/../nginx.run"

    [ "$status" -eq 0 ]
    [ "$(cat "$TMP/service.pid")" = "$(cat "$TMP/nginx.pid")" ]
}
