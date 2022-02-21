#!/usr/bin/env bash

# Data Path
DataPath=/usr/local/etc/nginx-ui
# Bin Path
# BinPath=/usr/local/bin/nginx-ui
# Service Path
ServicePath=/usr/lib/systemd/system/nginx-ui.service
PROXY=""

## Demo function for processing parameters
judgment_parameters() {
    while [[ "$#" -gt '0' ]]; do
        case "$1" in
        '-p' | '--proxy')
            if [[ -z "$2" ]]; then
                echo "error: Please specify the proxy server address."
                exit 1
            fi
            PROXY="$2"
            shift
            ;;
        esac
        shift
    done
}

check_if_running_as_root() {
    # If you want to run as another user, please modify $EUID to be owned by this user
    if [[ "$EUID" -ne '0' ]]; then
        echo "error: You must run this script as root!"
        exit 1
    fi
}

identify_the_operating_system_and_architecture() {
    if [[ "$(uname)" == 'Linux' ]]; then
        case "$(uname -m)" in
        'i386' | 'i686')
            MACHINE='386'
            ;;
        'amd64' | 'x86_64')
            MACHINE='amd64'
            ;;
        'armv8' | 'aarch64')
            MACHINE='arm64'
            ;;
        *)
            echo "error: The architecture is not supported."
            exit 1
            ;;
        esac
        if [[ ! -f '/etc/os-release' ]]; then
            echo "error: Don't use outdated Linux distributions."
            exit 1
        fi
        # Do not combine this judgment condition with the following judgment condition.
        ## Be aware of Linux distribution like Gentoo, which kernel supports switch between Systemd and OpenRC.
        if [[ -f /.dockerenv ]] || grep -q 'docker\|lxc' /proc/1/cgroup && [[ "$(type -P systemctl)" ]]; then
            true
        elif [[ -d /run/systemd/system ]] || grep -q systemd <(ls -l /sbin/init); then
            true
        else
            echo "error: Only Linux distributions using systemd are supported."
            exit 1
        fi
        if [[ "$(type -P apt)" ]]; then
            PACKAGE_MANAGEMENT_INSTALL='apt -y --no-install-recommends install'
            # PACKAGE_MANAGEMENT_REMOVE='apt purge'
        elif [[ "$(type -P dnf)" ]]; then
            PACKAGE_MANAGEMENT_INSTALL='dnf -y install'
            # PACKAGE_MANAGEMENT_REMOVE='dnf remove'
        elif [[ "$(type -P yum)" ]]; then
            PACKAGE_MANAGEMENT_INSTALL='yum -y install'
            # PACKAGE_MANAGEMENT_REMOVE='yum remove'
        elif [[ "$(type -P zypper)" ]]; then
            PACKAGE_MANAGEMENT_INSTALL='zypper install -y --no-recommends'
            # PACKAGE_MANAGEMENT_REMOVE='zypper remove'
        elif [[ "$(type -P pacman)" ]]; then
            PACKAGE_MANAGEMENT_INSTALL='pacman -Syu --noconfirm'
            # PACKAGE_MANAGEMENT_REMOVE='pacman -Rsn'
        else
            echo "error: The script does not support the package manager in this operating system."
            exit 1
        fi
    else
        echo "error: This operating system is not supported."
        exit 1
    fi
}

install_software() {
    package_name="$1"
    file_to_detect="$2"
    type -P "$file_to_detect" >/dev/null 2>&1 && return
    if ${PACKAGE_MANAGEMENT_INSTALL} "$package_name"; then
        echo "info: $package_name is installed."
    else
        echo "error: Installation of $package_name failed, please check your network."
        exit 1
    fi
}

download() {
    LATEST_RELEASE=$(curl -L -s --insecure -H 'Accept: application/json' "$PROXY"https://api.github.com/repos/0xJacky/nginx-ui/releases/latest)
    # shellcheck disable=SC2001
    LATEST_VERSION=$(echo "$LATEST_RELEASE" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    DOWNLOAD_LINK=$PROXY"https://github.com/0xJacky/nginx-ui/releases/download/$LATEST_VERSION/nginx-ui-linux-$MACHINE.tar.gz"

    echo "Downloading NginxUI archive: $DOWNLOAD_LINK"
    if ! curl --insecure -R -H 'Cache-Control: no-cache' -L "$DOWNLOAD_LINK" > "$TAR_FILE"; then
        echo 'error: Download failed! Please check your network or try again.'
        return 1
    fi
    return 0
}

decompression() {
    echo "$1"
    if ! tar -zxvf "$1" -C "$TMP_DIRECTORY"; then
        echo 'error: Nginx UI decompression failed.'
        "rm" -r "$TMP_DIRECTORY"
        echo "removed: $TMP_DIRECTORY"
        exit 1
    fi
    echo "info: Extract the Nginx UI package to $TMP_DIRECTORY and prepare it for installation."
}

install_bin() {
    NAME="nginx-ui"
    install -m 755 "${TMP_DIRECTORY}/$NAME" "/usr/local/bin/$NAME"
}

install_service() {
cat > "$ServicePath" << EOF
[Unit]
Description=Yet another WebUI for Nginx
Documentation=https://github.com/0xJacky/nginx-ui
After=network.target
[Service]
Type=simple
ExecStart=/usr/local/bin/nginx-ui --config /usr/local/etc/nginx-ui/app.ini
Restart=on-failure
TimeoutStopSec=5
KillMode=mixed
[Install]
WantedBy=multi-user.target
EOF
    chmod 644 ServicePath
}

start_nginx_ui() {
    if [[ -f ServicePath ]]; then
        systemctl start nginx-ui
        sleep 1s
        if systemctl -q is-active nginx-ui; then
            echo 'info: Start the Nginx UI service.'
        else
            echo 'error: Failed to start the Nginx UI service.'
            exit 1
        fi
    fi
}

stop_nginx_ui() {
    if ! systemctl stop nginx-ui; then
        echo 'error: Failed to stop the Nginx UI service.'
        exit 1
    fi
    echo 'info: Nginx UI service Stopped.'
}

main() {
    check_if_running_as_root

    judgment_parameters "$@"

    # TMP
    TMP_DIRECTORY="$(mktemp -d)"
    # Tar
    TAR_FILE="${TMP_DIRECTORY}/nginx-ui-linux-$MACHINE.tar.gz"

    identify_the_operating_system_and_architecture

    install_software 'curl' 'curl'

    download
    decompression "$TAR_FILE"

    install_bin
    install_service

    mkdir DataPath

    start_nginx_ui
    stop_nginx_ui

    systemctl start nginx-ui
    systemctl enable nginx-ui
    sleep 1s

    if systemctl -q is-active nginx-ui; then
        echo "info: Start and enable the Nginx UI service."
    else
        echo "warning: Failed to enable and start the Nginx UI service."
    fi
}

main "$@"
