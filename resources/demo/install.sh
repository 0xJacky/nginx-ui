#!/usr/bin/env bash

PROXY=""
RPROXY="https://ghproxy.com/"

MACHINE="amd64"

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
    install -m 755 "${TMP_DIRECTORY}/$NAME" "/app/$NAME"
}

main() {
    # Important Variables
    TMP_DIRECTORY="$(mktemp -d)"
    TAR_FILE="${TMP_DIRECTORY}/nginx-ui-linux-$MACHINE.tar.gz"
    get_latest_version
    download_nginx_ui
    decompression "$TAR_FILE"
    install_bin
}

main "$@"
