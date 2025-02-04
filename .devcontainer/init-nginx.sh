# init nginx config dir
if [ "$(ls -A /etc/nginx)" = "" ]; then
    echo "Initialing Nginx config dir"
    cp -rp /etc/nginx.orig/* /etc/nginx/
    echo "Initialed Nginx config dir"
fi