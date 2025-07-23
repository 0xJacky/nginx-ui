#!/usr/bin/env bash

# You can set this variable whatever you want in shell session right before running this script by issuing:
# export DATA_PATH='/usr/local/etc/nginx-ui'
DataPath=${DATA_PATH:-/usr/local/etc/nginx-ui}

# Service Path
ServicePath="/etc/systemd/system/nginx-ui.service"
# Init.d Path
InitPath="/etc/init.d/nginx-ui"
# OpenRC Path
OpenRCPath="/etc/init.d/nginx-ui"

# Service Type (systemd, openrc, initd)
SERVICE_TYPE=''

# Latest release version
RELEASE_LATEST=''

# Version channel (stable, prerelease, dev)
VERSION_CHANNEL='stable'

# install
INSTALL='0'

# remove
REMOVE='0'

# help
HELP='0'

# --local ?
LOCAL_FILE=''

# --proxy ?
PROXY=''

# --reverse-proxy ?
# You can set this variable whatever you want in shell session right before running this script by issuing:
# export GH_PROXY='https://cloud.nginxui.com/'
RPROXY=$GH_PROXY

# --purge
PURGE='0'

# Font color
FontBlack="\033[30m";
FontRed="\033[31m";
FontGreen="\033[32m";
FontYellow="\033[33m";
FontBlue="\033[34m";
FontPurple="\033[35m";
FontSkyBlue="\033[36m";
FontWhite="\033[37m";
FontSuffix="\033[0m";

curl_with_retry() {
    $(type -P curl) -x "${PROXY}" -L -q --retry 5 --retry-delay 10 --retry-max-time 60 "$@"
}

## Demo function for processing parameters
judgment_parameters() {
    while [[ "$#" -gt '0' ]]; do
        case "$1" in
        'install')
            INSTALL='1'
            ;;
        'remove')
            REMOVE='1'
            ;;
        'help')
            HELP='1'
            ;;
        '-l' | '--local')
            if [[ -z "$2" ]]; then
                echo "error: Please specify the correct local file."
                exit 1
            fi
            LOCAL_FILE="$2"
            shift
            ;;
        '-r' | '--reverse-proxy')
            if [[ -z "$2" ]]; then
                echo -e "${FontRed}error: Please specify the reverse proxy server address.${FontSuffix}"
                exit 1
            fi
            RPROXY="$2"
            shift
            ;;
        '-p' | '--proxy')
            if [[ -z "$2" ]]; then
                echo -e "${FontRed}error: Please specify the proxy server address.${FontSuffix}"
                exit 1
            fi
            PROXY="$2"
            shift
            ;;
        '-c' | '--channel')
            if [[ -z "$2" ]]; then
                echo -e "${FontRed}error: Please specify the version channel (stable, prerelease, dev).${FontSuffix}"
                exit 1
            fi
            if [[ "$2" != "stable" && "$2" != "prerelease" && "$2" != "dev" ]]; then
                echo -e "${FontRed}error: Invalid channel. Must be one of: stable, prerelease, dev.${FontSuffix}"
                exit 1
            fi
            VERSION_CHANNEL="$2"
            shift
            ;;
        '--purge')
            PURGE='1'
            ;;
        *)
            echo -e "${FontRed}$0: unknown option $1${FontSuffix}"
            exit 1
            ;;
        esac
        shift
    done
    if [ "$(expr $INSTALL + $HELP + $REMOVE)" -eq 0 ]; then
        INSTALL='1'
    elif [ "$(expr $INSTALL + $HELP + $REMOVE)" -gt 1 ]; then
        echo 'You can only choose one action.'
        exit 1
    fi
}

cat_file_with_name() {
    while [[ "$#" -gt '0' ]]; do
        echo -e "${FontSkyBlue}# $1${FontSuffix}\n"
        cat "$1"
        echo ''
        shift
    done
}

systemd_cat_config() {
    if systemd-analyze --help | grep -qw 'cat-config'; then
        systemd-analyze --no-pager cat-config "$@"
        echo
    else
        cat_file_with_name "$@" "$1".d/*
        echo -e "${FontYellow}warning: The systemd version on the current operating system is too low."
        echo -e "${FontYellow}warning: Please consider to upgrade the systemd or the operating system.${FontSuffix}"
        echo
    fi
}

check_if_running_as_root() {
    # If you want to run as another user, please modify $EUID to be owned by this user
    if [ "$(id -u)" != "0" ]; then
        echo -e "${FontRed}error: You must run this script as root!${FontSuffix}"
        exit 1
    fi
}

identify_the_operating_system_and_architecture() {
    if [[ "$(uname)" == 'Linux' ]]; then
        case "$(uname -m)" in
        'i386' | 'i686')
            MACHINE='32'
            ;;
        'amd64' | 'x86_64')
            MACHINE='64'
            ;;
        'armv5tel')
            MACHINE='arm32-v5'
            ;;
        'armv6l')
            MACHINE='arm32-v6'
            grep Features /proc/cpuinfo | grep -qw 'vfp' || MACHINE='arm32-v5'
            ;;
        'armv7' | 'armv7l')
            MACHINE='arm32-v7a'
            grep Features /proc/cpuinfo | grep -qw 'vfp' || MACHINE='arm32-v5'
            ;;
        'armv8' | 'aarch64')
            MACHINE='arm64-v8a'
            ;;
        *)
            echo -e "${FontRed}error: The architecture is not supported by this script.${FontSuffix}"
            exit 1
            ;;
        esac
        if [[ ! -f '/etc/os-release' ]]; then
            echo -e "${FontRed}error: Don't use outdated Linux distributions.${FontSuffix}"
            exit 1
        fi

        if [[ "$(type -P apt)" ]]; then
            PACKAGE_MANAGEMENT_INSTALL='apt -y --no-install-recommends install'
            PACKAGE_MANAGEMENT_REMOVE='apt purge'
        elif [[ "$(type -P dnf)" ]]; then
            PACKAGE_MANAGEMENT_INSTALL='dnf -y install'
            PACKAGE_MANAGEMENT_REMOVE='dnf remove'
        elif [[ "$(type -P yum)" ]]; then
            PACKAGE_MANAGEMENT_INSTALL='yum -y install'
            PACKAGE_MANAGEMENT_REMOVE='yum remove'
        elif [[ "$(type -P zypper)" ]]; then
            PACKAGE_MANAGEMENT_INSTALL='zypper install -y --no-recommends'
            PACKAGE_MANAGEMENT_REMOVE='zypper remove'
        elif [[ "$(type -P pacman)" ]]; then
            PACKAGE_MANAGEMENT_INSTALL='pacman -Syu --noconfirm'
            PACKAGE_MANAGEMENT_REMOVE='pacman -Rsn'
        elif [[ "$(type -P opkg)" ]]; then
            PACKAGE_MANAGEMENT_INSTALL='opkg install'
            PACKAGE_MANAGEMENT_REMOVE='opkg remove'
        elif [[ "$(type -P apk)" ]]; then
            PACKAGE_MANAGEMENT_INSTALL='apk add --no-cache'
            PACKAGE_MANAGEMENT_REMOVE='apk del'
        else
            echo -e "${FontRed}error: This script does not support the package manager in this operating system.${FontSuffix}"
            exit 1
        fi

        # Do not combine this judgment condition with the following judgment condition.
        ## Be aware of Linux distribution like Gentoo, which kernel supports switch between Systemd and OpenRC.
        if [[ -f /.dockerenv ]] || grep -q 'docker\|lxc' /proc/1/cgroup && [[ "$(type -P systemctl)" ]]; then
            SERVICE_TYPE='systemd'
        elif [[ -d /run/systemd/system ]] || grep -q systemd <(ls -l /sbin/init); then
            SERVICE_TYPE='systemd'
        elif [[ "$(type -P rc-update)" ]] || [[ "$(type -P apk)" ]]; then
            SERVICE_TYPE='openrc'
        else
            SERVICE_TYPE='initd'
            echo -e "${FontYellow}warning: No systemd or OpenRC detected, falling back to init.d.${FontSuffix}"
        fi
    else
        echo -e "${FontRed}error: This operating system is not supported by this script.${FontSuffix}"
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
        echo -e "${FontRed}error: Installation of $package_name failed, please check your network.${FontSuffix}"
        exit 1
    fi
}

get_latest_version() {
    # Get latest release version number
    local latest_release
    if [[ "$VERSION_CHANNEL" == "stable" ]]; then
        if ! latest_release=$(curl_with_retry -sS -H "Accept: application/vnd.github.v3+json" "https://api.github.com/repos/0xJacky/nginx-ui/releases/latest"); then
            echo -e "${FontRed}error: Failed to get release list, please check your network.${FontSuffix}"
            exit 1
        fi
        RELEASE_LATEST="$(echo "$latest_release" | sed 'y/,/\n/' | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')"
    elif [[ "$VERSION_CHANNEL" == "prerelease" ]]; then
        if ! latest_release=$(curl_with_retry -sS -H "Accept: application/vnd.github.v3+json" "https://api.github.com/repos/0xJacky/nginx-ui/releases"); then
            echo -e "${FontRed}error: Failed to get release list, please check your network.${FontSuffix}"
            exit 1
        fi
        # Find the latest prerelease version
        RELEASE_LATEST="$(echo "$latest_release" | sed 'y/,/\n/' | grep -B5 -A5 '"prerelease": true' | grep '"tag_name":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')"
        if [[ -z "$RELEASE_LATEST" ]]; then
            echo -e "${FontYellow}warning: No prerelease version found, falling back to stable version.${FontSuffix}"
            # Fallback to stable release
            if ! latest_release=$(curl_with_retry -sS -H "Accept: application/vnd.github.v3+json" "https://api.github.com/repos/0xJacky/nginx-ui/releases/latest"); then
                echo -e "${FontRed}error: Failed to get release list, please check your network.${FontSuffix}"
                exit 1
            fi
            RELEASE_LATEST="$(echo "$latest_release" | sed 'y/,/\n/' | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')"
        fi
    elif [[ "$VERSION_CHANNEL" == "dev" ]]; then
        # Get latest dev commit info
        local dev_commit
        if ! dev_commit=$(curl_with_retry -sS -H "Accept: application/vnd.github.v3+json" "${RPROXY}https://api.github.com/repos/0xJacky/nginx-ui/commits/dev?per_page=1"); then
            echo -e "${FontRed}error: Failed to get dev commit info, please check your network.${FontSuffix}"
            exit 1
        fi
        local commit_sha="$(echo "$dev_commit" | sed 'y/,/\n/' | grep '"sha":' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')"
        if [[ -z "$commit_sha" ]]; then
            echo -e "${FontRed}error: Failed to get dev commit SHA.${FontSuffix}"
            exit 1
        fi
        RELEASE_LATEST="sha-${commit_sha:0:7}"
    fi

    if [[ -z "$RELEASE_LATEST" ]]; then
        if echo "$latest_release" | grep -q "API rate limit exceeded"; then
            echo -e "${FontRed}error: github API rate limit exceeded${FontSuffix}"
        else
            echo -e "${FontRed}error: Failed to get the latest release version.${FontSuffix}"
            echo "Welcome bug report: https://github.com/0xJacky/nginx-ui/issues"
        fi
        exit 1
    fi
    RELEASE_LATEST="v${RELEASE_LATEST#v}"
}

download_nginx_ui() {
    local download_link
    if [[ "$VERSION_CHANNEL" == "dev" ]]; then
        # For dev builds, use the CloudflareWorkerAPI dev-builds endpoint
        download_link="https://cloud.nginxui.com/dev-builds/nginx-ui-linux-$MACHINE.tar.gz"
    else
        # For stable and prerelease versions
        download_link="${RPROXY}https://github.com/0xJacky/nginx-ui/releases/download/$RELEASE_LATEST/nginx-ui-linux-$MACHINE.tar.gz"
    fi

    echo "Downloading Nginx UI archive: $download_link"
    if ! curl_with_retry -R -H 'Cache-Control: no-cache' -L -o "$TAR_FILE" "$download_link"; then
        echo 'error: Download failed! Please check your network or try again.'
        return 1
    fi
    return 0
}

decompression() {
    echo "$1"
    if ! tar -zxf "$1" -C "$TMP_DIRECTORY"; then
        echo -e "${FontRed}error: Nginx UI decompression failed.${FontSuffix}"
        "rm" -r "$TMP_DIRECTORY"
        echo "removed: $TMP_DIRECTORY"
        exit 1
    fi
    echo "info: Extract the Nginx UI package to $TMP_DIRECTORY and prepare it for installation."
}

install_bin() {
    NAME="nginx-ui"
    
    if command -v install >/dev/null 2>&1; then
        install -m 755 "${TMP_DIRECTORY}/$NAME" "/usr/local/bin/$NAME"
    else
        cp "${TMP_DIRECTORY}/$NAME" "/usr/bin/$NAME"
        chmod 755 "/usr/bin/$NAME"
    fi
}

install_service() {
    if [[ "$SERVICE_TYPE" == "systemd" ]]; then
        install_systemd_service
    elif [[ "$SERVICE_TYPE" == "openrc" ]]; then
        install_openrc_service
    else
        install_initd_service
    fi
}

install_systemd_service() {
    mkdir -p '/etc/systemd/system/nginx-ui.service.d'
    local service_download_link="${RPROXY}https://raw.githubusercontent.com/0xJacky/nginx-ui/main/resources/services/nginx-ui.service"

    echo "Downloading Nginx UI service file: $service_download_link"
    if ! curl_with_retry -R -H 'Cache-Control: no-cache' -L -o "$ServicePath" "$service_download_link"; then
        echo -e "${FontRed}error: Download service file failed! Please check your network or try again.${FontSuffix}"
        return 1
    fi

    chmod 644 "$ServicePath"
    echo "info: Systemd service files have been installed successfully!"
    echo -e "${FontGreen}note: The following are the actual parameters for the nginx-ui service startup."
    echo -e "${FontGreen}note: Please make sure the configuration file path is correctly set.${FontSuffix}"
    systemd_cat_config "$ServicePath"
    systemctl daemon-reload
    SYSTEMD='1'
}

install_openrc_service() {
    local openrc_download_link="${RPROXY}https://raw.githubusercontent.com/0xJacky/nginx-ui/main/resources/services/nginx-ui.rc"

    echo "Downloading Nginx UI OpenRC file: $openrc_download_link"
    if ! curl_with_retry -R -H 'Cache-Control: no-cache' -L -o "$OpenRCPath" "$openrc_download_link"; then
        echo -e "${FontRed}error: Download OpenRC file failed! Please check your network or try again.${FontSuffix}"
        return 1
    fi

    chmod 755 "$OpenRCPath"
    echo "info: OpenRC service file has been installed successfully!"
    echo -e "${FontGreen}note: The OpenRC service is installed to '$OpenRCPath'.${FontSuffix}"
    cat_file_with_name "$OpenRCPath"

    # Add to default runlevel
    rc-update add nginx-ui default

    OPENRC='1'
}

install_initd_service() {
    # Download init.d script
    local initd_download_link="${RPROXY}https://raw.githubusercontent.com/0xJacky/nginx-ui/main/resources/services/nginx-ui.init"

    echo "Downloading Nginx UI init.d file: $initd_download_link"
    if ! curl_with_retry -R -H 'Cache-Control: no-cache' -L -o "$InitPath" "$initd_download_link"; then
        echo -e "${FontRed}error: Download init.d file failed! Please check your network or try again.${FontSuffix}"
        exit 1
    fi

    chmod 755 "$InitPath"
    echo "info: Init.d service file has been installed successfully!"
    echo -e "${FontGreen}note: The init.d service is installed to '$InitPath'.${FontSuffix}"
    cat_file_with_name "$InitPath"

    # Add service to startup based on distro
    if [ -x /sbin/chkconfig ]; then
        /sbin/chkconfig --add nginx-ui
    elif [ -x /usr/sbin/update-rc.d ]; then
        /usr/sbin/update-rc.d nginx-ui defaults
    fi

    INITD='1'
}

install_config() {
    mkdir -p "$DataPath"
    if [[ ! -f "$DataPath/app.ini" ]]; then
cat > "$DataPath/app.ini" << EOF
[app]
PageSize = 10

[server]
Host = 0.0.0.0
Port = 9000
RunMode = release

[cert]
HTTPChallengePort = 9180

[terminal]
StartCmd = login
EOF
        echo "info: The default configuration file was installed to '$DataPath/app.ini' successfully!"
    fi

    echo -e "${FontGreen}note: The following are the current configuration for the nginx-ui."
    echo -e "${FontGreen}note: Please change the information if needed.${FontSuffix}"
    cat_file_with_name "$DataPath/app.ini"
}

start_nginx_ui() {
    if [[ "$SERVICE_TYPE" == "systemd" ]]; then
        systemctl start nginx-ui
        sleep 1s
        if systemctl -q is-active nginx-ui; then
            echo 'info: Start the Nginx UI service.'
        else
            echo -e "${FontRed}error: Failed to start the Nginx UI service.${FontSuffix}"
            exit 1
        fi
    elif [[ "$SERVICE_TYPE" == "openrc" ]]; then
        # Check if service is already running
        if rc-service nginx-ui status | grep -qE "(started|running)"; then
            echo 'info: Nginx UI service is already running.'
        else
            rc-service nginx-ui start
            sleep 1s
            if rc-service nginx-ui status | grep -qE "(started|running)"; then
                echo 'info: Start the Nginx UI service.'
            else
                echo -e "${FontRed}error: Failed to start the Nginx UI service.${FontSuffix}"
                exit 1
            fi
        fi
    else
        # init.d
        $InitPath start
        sleep 1s
        if $InitPath status >/dev/null 2>&1; then
            echo 'info: Start the Nginx UI service.'
        else
            echo -e "${FontRed}error: Failed to start the Nginx UI service.${FontSuffix}"
            exit 1
        fi
    fi
}

check_nginx_ui_status() {
    if [[ "$SERVICE_TYPE" == "systemd" ]]; then
        if systemctl list-unit-files | grep -qw 'nginx-ui'; then
            if systemctl -q is-active nginx-ui; then
                return 0  # running
            else
                return 1  # not running
            fi
        else
            return 2  # not installed
        fi
    elif [[ "$SERVICE_TYPE" == "openrc" ]]; then
        if [[ -f "$OpenRCPath" ]]; then
            # Check if service is running using multiple methods
            if rc-service nginx-ui status | grep -qE "(started|running)" || [[ -n "$(pidof nginx-ui)" ]]; then
                return 0  # running
            else
                return 1  # not running
            fi
        else
            return 2  # not installed
        fi
    else
        # init.d
        if [[ -f "$InitPath" ]]; then
            if $InitPath status >/dev/null 2>&1; then
                return 0  # running
            else
                return 1  # not running
            fi
        else
            return 2  # not installed
        fi
    fi
}

restart_nginx_ui() {
    if [[ "$SERVICE_TYPE" == "systemd" ]]; then
        systemctl restart nginx-ui
        sleep 1s
        if systemctl -q is-active nginx-ui; then
            echo 'info: Restart the Nginx UI service.'
        else
            echo -e "${FontRed}error: Failed to restart the Nginx UI service.${FontSuffix}"
            exit 1
        fi
    elif [[ "$SERVICE_TYPE" == "openrc" ]]; then
        rc-service nginx-ui restart
        sleep 1s
        if rc-service nginx-ui status | grep -qE "(started|running)"; then
            echo 'info: Restart the Nginx UI service.'
        else
            echo -e "${FontRed}error: Failed to restart the Nginx UI service.${FontSuffix}"
            exit 1
        fi
    else
        # init.d
        $InitPath restart
        sleep 1s
        if $InitPath status >/dev/null 2>&1; then
            echo 'info: Restart the Nginx UI service.'
        else
            echo -e "${FontRed}error: Failed to restart the Nginx UI service.${FontSuffix}"
            exit 1
        fi
    fi
}

stop_nginx_ui() {
    if [[ "$SERVICE_TYPE" == "systemd" ]]; then
        if ! systemctl stop nginx-ui; then
            echo -e "${FontRed}error: Failed to stop the Nginx UI service.${FontSuffix}"
            exit 1
        fi
    elif [[ "$SERVICE_TYPE" == "openrc" ]]; then
        if ! rc-service nginx-ui stop; then
            echo -e "${FontRed}error: Failed to stop the Nginx UI service.${FontSuffix}"
            exit 1
        fi
    else
        # init.d
        if ! $InitPath stop; then
            echo -e "${FontRed}error: Failed to stop the Nginx UI service.${FontSuffix}"
            exit 1
        fi
    fi
    echo "info: Nginx UI service Stopped."
}

remove_nginx_ui() {
  if [[ "$SERVICE_TYPE" == "systemd" ]] && (systemctl list-unit-files | grep -qw 'nginx-ui' || [[ -f "/usr/local/bin/nginx-ui" ]]); then
    if [[ -n "$(pidof nginx-ui)" ]]; then
      stop_nginx_ui
    fi
    delete_files="/usr/local/bin/nginx-ui /etc/systemd/system/nginx-ui.service /etc/systemd/system/nginx-ui.service.d"
    if [[ "$PURGE" -eq '1' ]]; then
        [[ -d "$DataPath" ]] && delete_files="$delete_files $DataPath"
    fi
    systemctl disable nginx-ui 2>/dev/null || true
    if ! ("rm" -r $delete_files 2>/dev/null); then
      echo -e "${FontRed}error: Failed to remove Nginx UI.${FontSuffix}"
      exit 1
    else
      for file in $delete_files
      do
        [[ -e "$file" ]] && echo "removed: $file"
      done
      systemctl daemon-reload
      echo "You may need to execute a command to remove dependent software: $PACKAGE_MANAGEMENT_REMOVE curl"
      echo 'info: Nginx UI has been removed.'
      if [[ "$PURGE" -eq '0' ]]; then
        echo 'info: If necessary, manually delete the configuration and log files.'
        echo "info: e.g., $DataPath ..."
      fi
      exit 0
    fi
  elif [[ "$SERVICE_TYPE" == "openrc" ]] && ([[ -f "$OpenRCPath" ]] || [[ -f "/usr/local/bin/nginx-ui" ]]); then
    if rc-service nginx-ui status | grep -qE "(started|running)"; then
      stop_nginx_ui
    fi
    delete_files="/usr/local/bin/nginx-ui $OpenRCPath"
    if [[ "$PURGE" -eq '1' ]]; then
        [[ -d "$DataPath" ]] && delete_files="$delete_files $DataPath"
    fi

    # Remove from runlevels
    rc-update del nginx-ui default 2>/dev/null || true

    if ! ("rm" -r $delete_files 2>/dev/null); then
      echo -e "${FontRed}error: Failed to remove Nginx UI.${FontSuffix}"
      exit 1
    else
      for file in $delete_files
      do
        [[ -e "$file" ]] && echo "removed: $file"
      done
      echo "You may need to execute a command to remove dependent software: $PACKAGE_MANAGEMENT_REMOVE curl"
      echo 'info: Nginx UI has been removed.'
      if [[ "$PURGE" -eq '0' ]]; then
        echo 'info: If necessary, manually delete the configuration and log files.'
        echo "info: e.g., $DataPath ..."
      fi
      exit 0
    fi
  elif [[ "$SERVICE_TYPE" == "initd" ]] && ([[ -f "$InitPath" ]] || [[ -f "/usr/local/bin/nginx-ui" ]]); then
    if [[ -n "$(pidof nginx-ui)" ]]; then
      stop_nginx_ui
    fi
    delete_files="/usr/local/bin/nginx-ui $InitPath"
    if [[ "$PURGE" -eq '1' ]]; then
        [[ -d "$DataPath" ]] && delete_files="$delete_files $DataPath"
    fi

    # Remove from startup based on distro
    if [ -x /sbin/chkconfig ]; then
        /sbin/chkconfig --del nginx-ui 2>/dev/null || true
    elif [ -x /usr/sbin/update-rc.d ]; then
        /usr/sbin/update-rc.d -f nginx-ui remove 2>/dev/null || true
    fi

    if ! ("rm" -r $delete_files 2>/dev/null); then
      echo -e "${FontRed}error: Failed to remove Nginx UI.${FontSuffix}"
      exit 1
    else
      for file in $delete_files
      do
        [[ -e "$file" ]] && echo "removed: $file"
      done
      echo "You may need to execute a command to remove dependent software: $PACKAGE_MANAGEMENT_REMOVE curl"
      echo 'info: Nginx UI has been removed.'
      if [[ "$PURGE" -eq '0' ]]; then
        echo 'info: If necessary, manually delete the configuration and log files.'
        echo "info: e.g., $DataPath ..."
      fi
      exit 0
    fi
  else
    echo 'error: Nginx UI is not installed.'
    exit 1
  fi
}

# Explanation of parameters in the script
show_help() {
    echo "usage: $0 ACTION [OPTION]..."
    echo
    echo 'ACTION:'
    echo '  install                   Install/Update Nginx UI'
    echo '  remove                    Remove Nginx UI'
    echo '  help                      Show help'
    echo 'If no action is specified, then install will be selected'
    echo
    echo 'OPTION:'
    echo '  install:'
    echo '    -l, --local               Install Nginx UI from a local file'
    echo '    -p, --proxy               Download through a proxy server, e.g., -p http://127.0.0.1:8118 or -p socks5://127.0.0.1:1080'
    echo '    -r, --reverse-proxy       Download through a reverse proxy server, e.g., -r https://cloud.nginxui.com/'
    echo '    -c, --channel             Specify the version channel (stable, prerelease, dev)'
    echo '                              stable: Latest stable release (default)'
    echo '                              prerelease: Latest prerelease version'
    echo '                              dev: Latest development build from dev branch'
    echo '  remove:'
    echo '    --purge                   Remove all the Nginx UI files, include logs, configs, etc'
    exit 0
}

main() {
    check_if_running_as_root
    identify_the_operating_system_and_architecture
    judgment_parameters "$@"

    # Parameter information
    [[ "$HELP" -eq '1' ]] && show_help
    [[ "$REMOVE" -eq '1' ]] && remove_nginx_ui

    # Important Variables
    TMP_DIRECTORY="$(mktemp -d)"
    TAR_FILE="${TMP_DIRECTORY}/nginx-ui-linux-$MACHINE.tar.gz"

    # Auto install OpenRC on Alpine Linux if needed
    if [[ "$(type -P apk)" ]]; then
        install_software 'openrc' 'openrc'
    fi
    install_software 'curl' 'curl'

    # Install from a local file
    if [[ -n "$LOCAL_FILE" ]]; then
        echo "info: Install Nginx UI from a local file '$LOCAL_FILE'."
        decompression "$LOCAL_FILE"
    else
        get_latest_version
        echo "info: Installing Nginx UI $RELEASE_LATEST ($VERSION_CHANNEL channel) for $(uname -m)"
        if ! download_nginx_ui; then
            "rm" -r "$TMP_DIRECTORY"
            echo "removed: $TMP_DIRECTORY"
            exit 1
        fi
        decompression "$TAR_FILE"
    fi

    install_bin
    echo 'installed: /usr/local/bin/nginx-ui'

    install_service
    if [[ "$SERVICE_TYPE" == "systemd" && "$SYSTEMD" -eq '1' ]]; then
        echo "installed: ${ServicePath}"
    elif [[ "$SERVICE_TYPE" == "openrc" && "$OPENRC" -eq '1' ]]; then
        echo "installed: ${OpenRCPath}"
    elif [[ "$SERVICE_TYPE" == "initd" && "$INITD" -eq '1' ]]; then
        echo "installed: ${InitPath}"
    fi

    "rm" -r "$TMP_DIRECTORY"
    echo "removed: $TMP_DIRECTORY"
    echo "info: Nginx UI $RELEASE_LATEST is installed."

    install_config

    # Check nginx-ui service status and decide whether to start or restart
    check_nginx_ui_status
    service_status=$?
    
    if [[ $service_status -eq 0 ]]; then
        # Service is running, restart it
        echo "info: Nginx UI service is running, restarting..."
        restart_nginx_ui
    elif [[ $service_status -eq 1 ]]; then
        # Service is installed but not running, start it
        echo "info: Nginx UI service is not running, starting..."
        start_nginx_ui
        # Enable service for auto-start
        if [[ "$SERVICE_TYPE" == "systemd" ]]; then
            systemctl enable nginx-ui
        elif [[ "$SERVICE_TYPE" == "openrc" ]]; then
            rc-update add nginx-ui default
        fi
    else
        # Service is not installed, start it and enable
        echo "info: Installing and starting Nginx UI service..."
        if [[ "$SERVICE_TYPE" == "systemd" ]]; then
            systemctl start nginx-ui
            systemctl enable nginx-ui
            sleep 1s
            if systemctl -q is-active nginx-ui; then
                echo "info: Start and enable the Nginx UI service."
            else
                echo -e "${FontYellow}warning: Failed to enable and start the Nginx UI service.${FontSuffix}"
            fi
        elif [[ "$SERVICE_TYPE" == "openrc" ]]; then
            rc-service nginx-ui start
            rc-update add nginx-ui default
            sleep 1s
            if rc-service nginx-ui status | grep -qE "(started|running)"; then
                echo "info: Started and added the Nginx UI service to default runlevel."
            else
                echo -e "${FontYellow}warning: Failed to start the Nginx UI service.${FontSuffix}"
            fi
        elif [[ "$SERVICE_TYPE" == "initd" ]]; then
            $InitPath start
            sleep 1s
            if $InitPath status >/dev/null 2>&1; then
                echo "info: Started the Nginx UI service."
            else
                echo -e "${FontYellow}warning: Failed to start the Nginx UI service.${FontSuffix}"
            fi
        fi
    fi
}

main "$@"
