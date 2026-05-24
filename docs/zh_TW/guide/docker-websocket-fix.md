# 持久化 Docker 部署的 WebSocket 修復

::: tip 適用範圍
你以 Docker 卷的形式持久化了 `/etc/nginx`，且鏡像版本早於引入本修復的版本；
同時 Nginx UI 前面還有另一層反向代理（host nginx、Cloudflare、Traefik 等）在
終止 TLS。
:::

## 症狀

WebSocket 連線（終端、日誌即時追蹤等）報同源驗證失敗。原因是容器內部 nginx
將 `X-Forwarded-Proto` 覆寫為自己的 `$scheme`（`http`），導致 HTTPS 部署下
同源驗證失效。

## 自動修復（推薦）

1. 開啟 **系統 → 自檢**。
2. 找到 **Bundled nginx-ui.conf 已包含 WebSocket 反代修復**。
3. 點擊 **嘗試修復**。原檔案旁會產生帶時間戳的 `.bak` 備份。

::: warning 修復失敗時
原檔案會自動從備份還原。錯誤訊息中包含備份路徑。請參考下文「手動修復」。
:::

## 手動修復

::: code-group

```nginx [conf.d/nginx-ui.conf 頂部新增]
map $http_x_forwarded_proto $forwarded_proto {
    default $http_x_forwarded_proto;
    ''      $scheme;
}
map $http_x_forwarded_host $forwarded_host {
    default $http_x_forwarded_host;
    ''      $http_host;
}
```

```diff [location / 內部替換]
-        proxy_set_header   X-Forwarded-Proto    $scheme;
-        proxy_set_header   X-Forwarded-Host     $http_host;
+        proxy_set_header   X-Forwarded-Proto    $forwarded_proto;
+        proxy_set_header   X-Forwarded-Host     $forwarded_host;
```

:::

儲存後執行 `docker exec <container> nginx -s reload`。

## 停用自動升級

::: info
在容器上設定 `NGINX_UI_PRESERVE_BUNDLED_CONF=true` 即可關閉啟動期自動升級；
UI 內的修復入口仍可使用。
:::
