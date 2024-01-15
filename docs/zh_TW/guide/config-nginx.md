# Nginx

在本節中，我們將介紹 Nginx UI 中關於 Nginx 控制命令、日誌路徑等參數的配置選項。

::: tip 提示
自 v2.0.0-beta.3 版本起，我們將 `nginx_log` 配置項改名為 `nginx`。
:::

## 日誌
Nginx 日誌對於監控、排查問題和維護您的 Web 伺服器至關重要。它們提供了有關伺服器性能、用戶行為和潛在問題的寶貴見解。

在本節中，我們將討論兩種主要類型的日誌：訪問日誌和錯誤日誌。

對於從 v1.5.2 或更早版本升級的 Nginx-UI Docker 用戶，在配置 `app.ini` 之前，您需要在 `nginx.conf`
中添加單獨的 `access_log` 和 `error_log` 指令。

在 Nginx-UI 容器中，`/var/log/nginx/access.log` 是一個指向 `/dev/stdout` 的符號鏈接，而 `/var/log/nginx/error.log`
是一個指向 `/dev/stderr` 的符號鏈接。這種設置允許您使用 `docker logs nginx-ui` 命令查看 Nginx 和 Nginx-UI 日誌。然而，這兩個設備不支持
`tail` 命令，因此有必要使用額外的日誌文件來記錄 Nginx 日誌。

範例：

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

之後，請在 `app.ini` 中設置 nginx 訪問日誌和錯誤日誌路徑，然後重新啟動 nginx-ui。

範例：

```ini
[nginx_log]
AccessLogPath = /var/log/nginx/access.local.log
ErrorLogPath = /var/log/nginx/error.local.log
```

### AccessLogPath

- 類型：`string`

此選項用於為 Nginx UI 設置 Nginx 訪問日誌的路徑，以便我們在線查看日誌內容。

::: tip 提示
在 v2 版本中，我們會讀取 `nginx -V` 命令的輸出，以獲取 Nginx 訪問日誌的默認路徑。

如果您需要設置不同的路徑，您可以使用此選項。
:::

### ErrorLogPath

- 類型：`string`

此選項用於為 Nginx UI 設置 Nginx 錯誤日誌的路徑，以便我們在線查看日誌內容。

::: tip 提示
在 v2 版本中，我們會讀取 `nginx -V` 命令的輸出，以獲取 Nginx 錯誤日誌的默認路徑。

如果您需要設置不同的路徑，您可以使用此選項。
:::

## 服務監控與控制

在本節中，我們將會介紹 Nginx UI 中關於 Nginx 服務的監控和控制命令的配置選項。

### ConfigDir
- 類型：`string`

此選項用於設置 Nginx 配置文件夾的路徑。

在 v2 版

本中，我們會讀取 `nginx -V` 命令的輸出，以獲取 Nginx 配置文件的默認路徑。

如果您需要覆蓋默認路徑，您可以使用此選項。

### PIDPath
- 類型：`string`

此選項用於設置 Nginx PID 文件的路徑。Nginx UI 將通過判斷該文件是否存在來判斷 Nginx 服務的運行狀態。

在 v2 版本中，我們會讀取 `nginx -V` 命令的輸出，以獲取 Nginx PID 文件的默認路徑。

如果您需要覆蓋默認路徑，您可以使用此選項。

### TestConfigCmd
- 類型：`string`
- 默認值：`nginx -t`

此選項用於設置 Nginx 測試配置的命令。

### ReloadCmd
- 類型：`string`
- 默認值：`nginx -s reload`

此選項用於設置 Nginx 重新加載配置的命令。

### RestartCmd
- 類型：`string`

::: tip 提示
我們建議使用 systemd 管理 Nginx 的用戶，將這個值設置為 `systemctl restart nginx`。
否則，當您在 Nginx UI 中重啟 Nginx 後，將無法在 systemctl 中獲取 Nginx 的準確狀態。
:::

若此選項為空，則 Nginx UI 將使用以下命令關閉 Nginx 服務：

```bash
start-stop-daemon --stop --quiet --oknodo --retry=TERM/30/KILL/5 --pidfile $PID
```

若無法從 `nginx -V` 中獲得 `--sbin-path` 路徑，則 Nginx UI 將使用以下命令開啟 Nginx 服務：

```bash
start-stop-daemon --start --quiet --pidfile $PID --exec $SBIN_PATH
```
