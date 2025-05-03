#!/bin/bash
# init nginx config dir
if [ "$(ls -A /etc/nginx)" = "" ]; then
    echo "Initialing Nginx config dir"
    cp -rp /etc/nginx.orig/* /etc/nginx/
    echo "Initialed Nginx config dir"
fi


src_dir="/usr/share/nginx/modules-available"
dest_dir="/etc/nginx/modules-enabled"

create_symlink() {
    local module_name=$1
    local weight=$2

    local target="$dest_dir/$weight-$module_name"
    local source="$src_dir/$module_name"

    ln -sf "$source" "$target"
    echo "Created symlink: $target -> $source"
}

modules=(
    "mod-http-ndk.conf 10"
    "mod-http-auth-pam.conf 50"
    "mod-http-cache-purge.conf 50"
    "mod-http-dav-ext.conf 50"
    "mod-http-echo.conf 50"
    "mod-http-fancyindex.conf 50"
    "mod-http-geoip.conf 50"
    "mod-http-geoip2.conf 50"
    "mod-http-headers-more-filter.conf 50"
    "mod-http-image-filter.conf 50"
    "mod-http-lua.conf 50"
    "mod-http-perl.conf 50"
    "mod-http-subs-filter.conf 50"
    "mod-http-uploadprogress.conf 50"
    "mod-http-upstream-fair.conf 50"
    "mod-http-xslt-filter.conf 50"
    "mod-mail.conf 50"
    "mod-nchan.conf 50"
    "mod-stream.conf 50"
    "mod-stream-geoip.conf 70"
    "mod-stream-geoip2.conf 70"
)

for module in "${modules[@]}"; do
    module_name=$(echo $module | awk '{print $1}')
    weight=$(echo $module | awk '{print $2}')

    create_symlink "$module_name" "$weight"
done

# start nginx
nginx
