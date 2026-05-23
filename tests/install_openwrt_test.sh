#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_SCRIPT="$ROOT_DIR/install.sh"

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
    cat > "$sandbox/etc/os-release" <<'EOF'
NAME="OpenWrt"
ID="openwrt"
VERSION_ID="25.10"
EOF
    : > "$sandbox/etc/openwrt_release"
    : > "$sandbox/proc/cpuinfo"
    : > "$sandbox/proc/1/cgroup"
    ln -s /bin/true "$sandbox/bin/apk"

    output="$(NGINX_UI_INSTALL_TESTING=1 "$INSTALL_SCRIPT" __test_detect "$sandbox" 2>&1)"

    assert_contains "$output" "PACKAGE_MANAGEMENT_INSTALL=apk add --no-cache" "OpenWrt 25 should use apk for packages"
    assert_contains "$output" "SERVICE_TYPE=openwrt" "OpenWrt 25 should use OpenWrt service type"
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
VERSION_ID="25.10"
EOF
    : > "$sandbox/etc/openwrt_release"
    : > "$sandbox/proc/cpuinfo"
    : > "$sandbox/proc/1/cgroup"
    ln -s /bin/true "$sandbox/bin/apk"

    output="$(PATH="$sandbox/bin:$PATH" NGINX_UI_INSTALL_TESTING=1 "$INSTALL_SCRIPT" __test_detect "$sandbox" 2>&1)"

    assert_contains "$output" "MACHINE=riscv64" "OpenWrt riscv64 should use the linux-riscv64 artifact"
    assert_contains "$output" "SERVICE_TYPE=openwrt" "OpenWrt riscv64 should use OpenWrt service type"
}

test_openwrt_with_apk_is_not_detected_as_openrc
echo "ok - install.sh detects OpenWrt 25 apk as openwrt"

test_openwrt_riscv64_is_supported
echo "ok - install.sh detects OpenWrt riscv64 as openwrt"
