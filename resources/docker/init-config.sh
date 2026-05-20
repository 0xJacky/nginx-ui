#!/bin/bash

if [ "${NGINX_UI_DISABLE_BUNDLED_NGINX}" = "true" ]; then
    echo "[INFO] host mode: skipping bundled nginx config initialization"
    exit 0
fi

if [ "$(ls -A /etc/nginx)" = "" ]; then
    cp -rp /usr/local/etc/nginx/* /etc/nginx/
    echo "[INFO] Nginx configurations directory initialized"
fi
