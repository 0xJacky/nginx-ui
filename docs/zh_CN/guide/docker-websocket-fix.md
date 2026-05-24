# 持久化 Docker 部署的 WebSocket 修复

::: tip 适用范围
你以 Docker 卷的形式持久化了 `/etc/nginx`，且镜像版本早于引入本修复的版本；
同时 Nginx UI 前面还有另一层反向代理（host nginx、Cloudflare、Traefik 等）在
终止 TLS。
:::

## 症状

WebSocket 连接（终端、日志实时跟踪等）报同源校验失败。原因是容器内部 nginx
将 `X-Forwarded-Proto` 覆盖为自己的 `$scheme`（`http`），导致 HTTPS 部署下
同源校验失效。

## 自动修复（推荐）

1. 打开 **系统 → 自检**。
2. 找到 **Bundled nginx-ui.conf 已包含 WebSocket 反代修复**。
3. 点击 **尝试修复**。原文件旁会生成带时间戳的 `.bak` 备份。

::: warning 修复失败时
原文件会自动从备份恢复。错误信息中包含备份路径。请参考下文「手动修复」。
:::

## 手动修复

::: code-group

```nginx [conf.d/nginx-ui.conf 顶部新增]
map $http_x_forwarded_proto $forwarded_proto {
    default $http_x_forwarded_proto;
    ''      $scheme;
}
map $http_x_forwarded_host $forwarded_host {
    default $http_x_forwarded_host;
    ''      $http_host;
}
```

```diff [location / 内部替换]
-        proxy_set_header   X-Forwarded-Proto    $scheme;
-        proxy_set_header   X-Forwarded-Host     $http_host;
+        proxy_set_header   X-Forwarded-Proto    $forwarded_proto;
+        proxy_set_header   X-Forwarded-Host     $forwarded_host;
```

:::

保存后执行 `docker exec <container> nginx -s reload`。

## 禁用自动升级

::: info
在容器上设置 `NGINX_UI_PRESERVE_BUNDLED_CONF=true` 可关闭启动期的自动升级；
UI 内的修复入口仍然可用。
:::
