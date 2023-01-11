#!/bin/bash

if [ "$(ls -A /etc/nginx)" = "" ]; then
    echo "Initialing Nginx config dir"
    cp -rp /usr/etc/nginx/* /etc/nginx/
    echo "Initialed Nginx config dir"
fi

nginx &
/app/nginx-ui --config /etc/nginx-ui/app.ini
