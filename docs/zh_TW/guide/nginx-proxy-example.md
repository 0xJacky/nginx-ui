# Nginx 反向代理示例

在本指南中，我們將引導您配置 Nginx 伺服器以將 HTTP 流量重定向到 HTTPS，併為監聽在 `http://127.0.0.1:9000/` 上的 Nginx UI
設定反向代理。

```nginx
server {
    listen          80;
    listen          [::]:80;

    server_name     <your_server_name>;
    rewrite ^(.*)$  https://$host$1 permanent;
}

map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen  443       ssl http2;
    listen  [::]:443  ssl http2;

    server_name         <your_server_name>;

    ssl_certificate     /path/to/ssl_cert;
    ssl_certificate_key /path/to/ssl_cert_key;

    location / {
        proxy_set_header    Host                $host;
        proxy_set_header    X-Real-IP           $remote_addr;
        proxy_set_header    X-Forwarded-For     $proxy_add_x_forwarded_for;
        proxy_set_header    X-Forwarded-Proto   $scheme;
        proxy_http_version  1.1;
        proxy_set_header    Upgrade             $http_upgrade;
        proxy_set_header    Connection          $connection_upgrade;
        proxy_pass          http://127.0.0.1:9000/;
    }
}
```

配置檔案包括兩個 Nginx 伺服器塊。第一個伺服器塊偵聽 80 埠（HTTP），並將所有傳入的 HTTP 請求重定向到 HTTPS。它還監聽 IPv6
地址。將 `<your_server_name>` 替換為您的伺服器名稱。

第二個伺服器塊監聽 443 埠（HTTPS）以及 HTTP/2 協議。同樣，它也監聽 IPv6 地址。將 `<your_server_name>` 替換為您的伺服器名稱，並將
SSL 證書和金鑰的路徑替換為 `/path/to/ssl_cert` 和 `/path/to/ssl_cert_key`。

此外，配置包括一個 `map` 指令，用於根據 `$http_upgrade` 變數設定 `$connection_upgrade` 變數的值，該變數用於 WebSocket 連線。

在第二個伺服器塊中，`location /` 部分包含代理設定，將請求轉發到本地埠 `9000`
。代理設定還包括一些用於正確處理轉發請求的信頭，如 `Host`、`X-Real-IP`、`X-Forwarded-For`、`X-Forwarded-Proto`、`Upgrade`
和 `Connection`。
