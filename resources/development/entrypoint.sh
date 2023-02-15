#!/bin/bash

if [ "$(ls -A /etc/nginx)" = "" ]; then
    echo "Initialing Nginx config dir"
    cp -rp /usr/etc/nginx/* /etc/nginx/
    echo "Initialed Nginx config dir"
fi

echo "export PATH=$PATH:/usr/local/go/bin:$(go env GOPATH)/bin" >> ~/.profile
source ~/.profile

nginx
cd /app && air
