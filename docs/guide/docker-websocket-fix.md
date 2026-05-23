# WebSocket fix for persisted Docker installations

::: tip Applies to
You persisted `/etc/nginx` as a Docker volume from a Nginx UI version older than
the one that introduced this fix, and Nginx UI is fronted by another reverse proxy
that terminates TLS (host nginx, Cloudflare, Traefik, ...).
:::

## Symptoms

WebSocket connections (terminal, log live tail, ...) fail with origin-mismatch errors.
This happens because the container's internal nginx was overwriting `X-Forwarded-Proto`
with its own `$scheme` (`http`), breaking the same-origin check on HTTPS deployments.

## Automatic fix (recommended)

1. Open **System → Self Check**.
2. Locate **Bundled nginx-ui.conf has WebSocket reverse-proxy fix**.
3. Click **Attempt to fix**. A timestamped `.bak` file is written next to the original.

::: warning If the fix fails
The original file is restored from backup automatically. The error message includes
the backup path. See *Manual fix* below.
:::

## Manual fix

::: code-group

```nginx [Additions at top of conf.d/nginx-ui.conf]
map $http_x_forwarded_proto $forwarded_proto {
    default $http_x_forwarded_proto;
    ''      $scheme;
}
map $http_x_forwarded_host $forwarded_host {
    default $http_x_forwarded_host;
    ''      $http_host;
}
```

```diff [Replace inside location /]
-        proxy_set_header   X-Forwarded-Proto    $scheme;
-        proxy_set_header   X-Forwarded-Host     $http_host;
+        proxy_set_header   X-Forwarded-Proto    $forwarded_proto;
+        proxy_set_header   X-Forwarded-Host     $forwarded_host;
```

:::

After saving, run `docker exec <container> nginx -s reload`.

## Opt-out

::: info
Set `NGINX_UI_PRESERVE_BUNDLED_CONF=true` on the container to disable the
startup-time auto-upgrade. The UI-driven fix remains available regardless.
:::
