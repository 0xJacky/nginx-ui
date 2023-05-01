server {
    listen 80;
    listen [::]:80;
    server_name test.nginxui.com;
    location /.well-known/acme-challenge {
        proxy_set_header Host $host;
        proxy_set_header X-Real_IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr:$remote_port;
        proxy_pass http://127.0.0.1:5002;
    }
}
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name test.nginxui.com;
    ssl_certificate /etc/nginx/ssl/test.nginxui.com/fullchain.cer;
    ssl_certificate_key /etc/nginx/ssl/test.nginxui.com/private.key;
    location /.well-known/acme-challenge {
        proxy_set_header Host $host;
        proxy_set_header X-Real_IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr:$remote_port;
        proxy_pass http://127.0.0.1:5002;
    }
}