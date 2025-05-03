#!/usr/bin/env bash

# You can set this variable whatever you want in shell session right before running this script by issuing:
# export DATA_PATH='/usr/local/etc/nginx-ui'
DataPath=${DATA_PATH:-/usr/local/etc/nginx-ui}

# Service Path
ServicePath="/etc/systemd/system/nginx-ui.service"

# Latest release version
RELEASE_LATEST=''

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

curl() {
    $(type -P curl) -L -q --retry 5 --retry-delay 10 --retry-max-time 60 "$@"
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
    if ((INSTALL+HELP+REMOVE==0)); then
        INSTALL='1'
    elif ((INSTALL+HELP+REMOVE>1)); then
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
    if [[ "$EUID" -ne '0' ]]; then
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
        # Do not combine this judgment condition with the following judgment condition.
        ## Be aware of Linux distribution like Gentoo, which kernel supports switch between Systemd and OpenRC.
        if [[ -f /.dockerenv ]] || grep -q 'docker\|lxc' /proc/1/cgroup && [[ "$(type -P systemctl)" ]]; then
            true
        elif [[ -d /run/systemd/system ]] || grep -q systemd <(ls -l /sbin/init); then
            true
        else
            echo -e "${FontRed}error: Only Linux distributions using systemd are supported by this script."
            echo -e "${FontRed}error: Please download the pre-built binary from the release page or build it manually.${FontSuffix}"
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
        else
            echo -e "${FontRed}error: This script does not support the package manager in this operating system.${FontSuffix}"
            exit 1
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
    if ! latest_release=$(curl -x "${PROXY}" -sS -H "Accept: application/vnd.github.v3+json" "https://api.github.com/repos/0xJacky/nginx-ui/releases/latest"); then
        echo -e "${FontRed}error: Failed to get release list, please check your network.${FontSuffix}"
        exit 1
    fi

    RELEASE_LATEST="$(echo "$latest_release" | sed 'y/,/\n/' | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')"
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
    download_link="${RPROXY}https://github.com/0xJacky/nginx-ui/releases/download/$RELEASE_LATEST/nginx-ui-linux-$MACHINE.tar.gz"

    echo "Downloading Nginx UI archive: $download_link"
    if ! curl -x "${PROXY}" -R -H 'Cache-Control: no-cache' -L -o "$TAR_FILE" "$download_link"; then
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
    install -m 755 "${TMP_DIRECTORY}/$NAME" "/usr/local/bin/$NAME"
}

install_service() {
    mkdir -p '/etc/systemd/system/nginx-ui.service.d'
cat > "$ServicePath" << EOF
[Unit]
Description=Yet another WebUI for Nginx
Documentation=https://github.com/0xJacky/nginx-ui
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/nginx-ui -config /usr/local/etc/nginx-ui/app.ini
RuntimeDirectory=nginx-ui
WorkingDirectory=/var/run/nginx-ui
Restart=on-failure
TimeoutStopSec=5
KillMode=mixed

[Install]
WantedBy=multi-user.target
EOF
    chmod 644 "$ServicePath"
    echo "info: Systemd service files have been installed successfully!"
    echo -e "${FontGreen}note: The following are the actual parameters for the nginx-ui service startup."
    echo -e "${FontGreen}note: Please make sure the configuration file path is correctly set.${FontSuffix}"
    systemd_cat_config "$ServicePath"
    systemctl daemon-reload
    SYSTEMD='1'
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
    if [[ -f "$ServicePath" ]]; then
        systemctl start nginx-ui
        sleep 1s
        if systemctl -q is-active nginx-ui; then
            echo 'info: Start the Nginx UI service.'
        else
            echo -e "${FontRed}error: Failed to start the Nginx UI service.${FontSuffix}"
            exit 1
        fi
    fi
}

stop_nginx_ui() {
    if ! systemctl stop nginx-ui; then
        echo -e "${FontRed}error: Failed to stop the Nginx UI service.${FontSuffix}"
        exit 1
    fi
    echo "info: Nginx UI service Stopped."
}

remove_nginx_ui() {
  if systemctl list-unit-files | grep -qw 'nginx-ui'; then
    if [[ -n "$(pidof nginx-ui)" ]]; then
      stop_nginx_ui
    fi
    local delete_files=('/usr/local/bin/nginx-ui' '/etc/systemd/system/nginx-ui.service' '/etc/systemd/system/nginx-ui.service.d')
    if [[ "$PURGE" -eq '1' ]]; then
        [[ -d "$DataPath" ]] && delete_files+=("$DataPath")
    fi
    systemctl disable nginx-ui
    if ! ("rm" -r "${delete_files[@]}"); then
      echo -e "${FontRed}error: Failed to remove Nginx UI.${FontSuffix}"
      exit 1
    else
      for i in "${!delete_files[@]}"
      do
        echo "removed: ${delete_files[$i]}"
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

    install_software 'curl' 'curl'

    # Install from a local file
    if [[ -n "$LOCAL_FILE" ]]; then
        echo "info: Install Nginx UI from a local file '$LOCAL_FILE'."
        decompression "$LOCAL_FILE"
    else
        get_latest_version
        echo "info: Installing Nginx UI $RELEASE_LATEST for $(uname -m)"
        if ! download_nginx_ui; then
            "rm" -r "$TMP_DIRECTORY"
            echo "removed: $TMP_DIRECTORY"
            exit 1
        fi
        decompression "$TAR_FILE"
    fi

    # Determine if nginx-ui is running
    if systemctl list-unit-files | grep -qw 'nginx-ui'; then
        if [[ -n "$(pidof nginx-ui)" ]]; then
            stop_nginx_ui
            NGINX_UI_RUNNING='1'
        fi
    fi
    install_bin
    echo 'installed: /usr/local/bin/nginx-ui'

    install_service
    if [[ "$SYSTEMD" -eq '1' ]]; then
        echo "installed: ${ServicePath}"
    fi

    "rm" -r "$TMP_DIRECTORY"
    echo "removed: $TMP_DIRECTORY"
    echo "info: Nginx UI $RELEASE_LATEST is installed."

    install_config

    if [[ "$NGINX_UI_RUNNING" -eq '1' ]]; then
        start_nginx_ui
    else
        systemctl start nginx-ui
        systemctl enable nginx-ui
        sleep 1s

        if systemctl -q is-active nginx-ui; then
            echo "info: Start and enable the Nginx UI service."
        else
            echo -e "${FontYellow}warning: Failed to enable and start the Nginx UI service.${FontSuffix}"
        fi
    fi
}

main "$@"
