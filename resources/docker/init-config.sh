#!/bin/bash

if [ "$(ls -A /etc/nginx)" = "" ]; then
    cp -rp /usr/local/etc/nginx/* /etc/nginx/
    echo "[INFO] Nginx configurations directory initialized"
fi
