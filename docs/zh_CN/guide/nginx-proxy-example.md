# Nginx 反向代理示例

在本指南中，我们将引导您配置 Nginx 服务器以将 HTTP 流量重定向到 HTTPS，并为监听在 `http://127.0.0.1:9000/` 上的 Nginx UI
设置反向代理。

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
    listen  443       ssl;
    listen  [::]:443  ssl;
    http2   on;

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

配置文件包括两个 Nginx 服务器块。第一个服务器块侦听 80 端口（HTTP），并将所有传入的 HTTP 请求重定向到 HTTPS。它还监听 IPv6
地址。将 `<your_server_name>` 替换为您的服务器名称。

第二个服务器块监听 443 端口（HTTPS）以及 HTTP/2 协议。同样，它也监听 IPv6 地址。将 `<your_server_name>` 替换为您的服务器名称，并将
SSL 证书和密钥的路径替换为 `/path/to/ssl_cert` 和 `/path/to/ssl_cert_key`。

此外，配置包括一个 `map` 指令，用于根据 `$http_upgrade` 变量设置 `$connection_upgrade` 变量的值，该变量用于 WebSocket 连接。

在第二个服务器块中，`location /` 部分包含代理设置，将请求转发到本地端口 `9000`
。代理设置还包括一些用于正确处理转发请求的标头，如 `Host`、`X-Real-IP`、`X-Forwarded-For`、`X-Forwarded-Proto`、`Upgrade`
和 `Connection`。
