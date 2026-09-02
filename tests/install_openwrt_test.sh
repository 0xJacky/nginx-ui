#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_SCRIPT="$ROOT_DIR/install.sh"
OPENWRT_SERVICE="$ROOT_DIR/resources/services/nginx-ui.openwrt"
OPENWRT_KEEP="$ROOT_DIR/resources/keep.openwrt"
OPENWRT_PACKAGE="$ROOT_DIR/packaging/openwrt/Makefile"
OPENWRT_CONFIG="$ROOT_DIR/packaging/openwrt/files/app.ini"

assert_contains() {
    local haystack="$1"
    local needle="$2"
    local message="$3"

    if [[ "$haystack" != *"$needle"* ]]; then
        echo "not ok - $message"
        echo "expected to find: $needle"
        echo "actual output:"
        echo "$haystack"
        exit 1
    fi
}

test_openwrt_with_apk_is_not_detected_as_openrc() {
    local sandbox output
    sandbox="$(mktemp -d)"
    trap 'rm -rf "$sandbox"' RETURN

    mkdir -p "$sandbox/bin" "$sandbox/etc" "$sandbox/proc/1" "$sandbox/sbin"
    cat > "$sandbox/bin/uname" <<'EOF'
#!/usr/bin/env bash

if [[ "${1:-}" == "-m" ]]; then
    echo 'aarch64'
else
    echo 'Linux'
fi
EOF
    chmod +x "$sandbox/bin/uname"

    cat > "$sandbox/etc/os-release" <<'EOF'
NAME="OpenWrt"
ID="openwrt"
VERSION_ID="25.12"
EOF
    : > "$sandbox/etc/openwrt_release"
    : > "$sandbox/proc/cpuinfo"
    : > "$sandbox/proc/1/cgroup"
    cat > "$sandbox/bin/apk" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
    chmod +x "$sandbox/bin/apk"

    output="$(PATH="$sandbox/bin:$PATH" NGINX_UI_INSTALL_TESTING=1 bash "$INSTALL_SCRIPT" __test_detect "$sandbox" v2.6.0 2>&1)"

    assert_contains "$output" "PACKAGE_MANAGEMENT_INSTALL=apk add --no-cache" "OpenWrt 25 should use apk for packages"
    assert_contains "$output" "SERVICE_TYPE=openwrt" "OpenWrt 25 should use OpenWrt service type"
    assert_contains "$output" "BINARY_PATH=/usr/bin/nginx-ui" "OpenWrt should install the binary under /usr/bin"
    assert_contains "$output" "DATA_PATH=/etc/nginx-ui" "OpenWrt should store configuration under /etc"
    assert_contains "$output" "SERVICE_RESOURCE_REF=v2.6.0" "release installs should pin service resources to the release tag"
}

test_openwrt_riscv64_is_supported() {
    local sandbox output
    sandbox="$(mktemp -d)"
    trap 'rm -rf "$sandbox"' RETURN

    mkdir -p "$sandbox/bin" "$sandbox/etc" "$sandbox/proc/1" "$sandbox/sbin"
    cat > "$sandbox/bin/uname" <<'EOF'
#!/usr/bin/env bash

if [[ "${1:-}" == "-m" ]]; then
    echo 'riscv64'
else
    echo 'Linux'
fi
EOF
    chmod +x "$sandbox/bin/uname"

    cat > "$sandbox/etc/os-release" <<'EOF'
NAME="OpenWrt"
ID="openwrt"
VERSION_ID="25.12"
EOF
    : > "$sandbox/etc/openwrt_release"
    : > "$sandbox/proc/cpuinfo"
    : > "$sandbox/proc/1/cgroup"
    cat > "$sandbox/bin/apk" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
    chmod +x "$sandbox/bin/apk"

    output="$(PATH="$sandbox/bin:$PATH" NGINX_UI_INSTALL_TESTING=1 bash "$INSTALL_SCRIPT" __test_detect "$sandbox" 2>&1)"

    assert_contains "$output" "MACHINE=riscv64" "OpenWrt riscv64 should use the linux-riscv64 artifact"
    assert_contains "$output" "SERVICE_TYPE=openwrt" "OpenWrt riscv64 should use OpenWrt service type"
}

test_openwrt_service_and_upgrade_paths_are_consistent() {
    local service keep package config
    service="$(cat "$OPENWRT_SERVICE")"
    keep="$(cat "$OPENWRT_KEEP")"
    package="$(cat "$OPENWRT_PACKAGE")"
    config="$(cat "$OPENWRT_CONFIG")"

    assert_contains "$service" 'PROG="/usr/bin/nginx-ui"' "procd should start the packaged binary"
    assert_contains "$service" 'CONFIG="/etc/nginx-ui/app.ini"' "procd should use the packaged configuration"
    assert_contains "$keep" "/etc/nginx-ui/" "sysupgrade should preserve Nginx UI configuration"
    assert_contains "$keep" "/etc/init.d/nginx-ui" "sysupgrade should preserve the service definition"
    assert_contains "$package" '$(1)/usr/bin/nginx-ui' "the APK should install the binary used by procd"
    assert_contains "$package" '$(1)/etc/nginx-ui/app.ini' "the APK should install the declared conffile"
    assert_contains "$package" '$(1)/lib/upgrade/keep.d/nginx-ui' "the APK should install the sysupgrade keep file"
    assert_contains "$config" "Host = 0.0.0.0" "the packaged backend should listen on the pod or router network"
}

test_openwrt_with_apk_is_not_detected_as_openrc
echo "ok - install.sh detects OpenWrt 25 apk as openwrt"

test_openwrt_riscv64_is_supported
echo "ok - install.sh detects OpenWrt riscv64 as openwrt"

test_openwrt_service_and_upgrade_paths_are_consistent
echo "ok - OpenWrt service and sysupgrade paths are consistent"
