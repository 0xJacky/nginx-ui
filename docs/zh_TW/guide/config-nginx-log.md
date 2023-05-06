# Nginx 日誌

Nginx 日誌對於監控、排查問題和維護您的 Web 伺服器至關重要。它們提供了有關伺服器效能、使用者行為和潛在問題的寶貴見解。在本節中，我們將討論兩種主要型別的日誌：訪問日誌和錯誤日誌。

對於從 v1.5.2 或更早版本升級的 Nginx-UI Docker 使用者，在配置 `app.ini` 之前，至關重要的是在您的 `nginx.conf`
中新增單獨的 `access_log` 和 `error_log` 指令。

在 Nginx-UI 容器中，`/var/log/nginx/access.log` 是一個指向 `/dev/stdout` 的符號連結，而 `/var/log/nginx/error.log`
是一個指向 `/dev/stderr` 的符號連結。這種設定允許您使用 `docker logs nginx-ui` 命令檢視 Nginx 和 Nginx-UI 日誌。然而，這兩個裝置不支援
`tail` 命令，因此有必要使用額外的日誌檔案來記錄 Nginx 日誌。

示例：

```nginx
error_log /var/log/nginx/error.log notice;
error_log /var/log/nginx/error.local.log notice;

http {
...
    access_log /var/log/nginx/access.log main;
    access_log /var/log/nginx/access.local.log main;
...
}
```

此外，在 `app.ini` 中設定 nginx 訪問日誌和錯誤日誌路徑，然後重新啟動 nginx-ui。

示例：

```ini
[nginx_log]
AccessLogPath = /var/log/nginx/access.local.log
ErrorLogPath = /var/log/nginx/error.local.log
```

## AccessLogPath

- 型別：`string`

此選項用於為 Nginx UI 設定 Nginx 訪問日誌的路徑，以便我們線上檢視日誌內容。

## ErrorLogPath

- 型別：`string`

此選項用於為 Nginx UI 設定 Nginx 錯誤日誌的路徑，以便我們線上檢視日誌內容。
