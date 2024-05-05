#!/bin/bash

# the up and down files are special: they're not shell scripts,
# but single command lines interpreted by execlineb.
# You should not have to worry about execline;
# you should only remember that an up file contains a single command line.

if [ "$(ls -A /etc/nginx)" = "" ]; then
    echo "[INFO] Initialing Nginx configurations directory"
    cp -rp /usr/etc/nginx/* /etc/nginx/
    echo "[INFO] Nginx configurations directory initialed"
fi
