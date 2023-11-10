# Example of Nginx Reverse Proxy

In this guide, we'll walk you through the process of configuring an Nginx server to redirect HTTP traffic to HTTPS and
set up a reverse proxy for the Nginx UI running on `http://127.0.0.1:9000/`.

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

The configuration file consists of two Nginx server blocks. The first server block listens on port 80 (HTTP) and
redirects all incoming HTTP requests to HTTPS. It also listens for IPv6 addresses. Replace `<your_server_name>` with
your
server name.

The second server block listens on port 443 (HTTPS) along with the HTTP/2 protocol. Again, it listens for IPv6 addresses
as well. Replace `<your_server_name>` with your server name and the paths for the SSL certificate and key with
`/path/to/ssl_cert` and `/path/to/ssl_cert_key`.

Additionally, the configuration includes a map directive for setting the value of the `$connection_upgrade` variable
based on the $http_upgrade variable, which is used for WebSocket connections.

Within the second server block, the location `/` section contains proxy settings to forward requests to the local port
`9000`. The proxy settings also include a number of headers for proper handling of the forwarded requests, such
as `Host`,
`X-Real-IP`, `X-Forwarded-For`, `X-Forwarded-Proto`, `Upgrade`, and `Connection`.
