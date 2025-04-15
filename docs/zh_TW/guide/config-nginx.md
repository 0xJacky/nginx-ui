# Nginx

在本節中，我們將介紹 Nginx UI 中關於 Nginx 控制命令、日誌路徑等參數的設定選項。

::: tip 提示
自 v2.0.0-beta.3 版本起，我們將 `nginx_log` 設定項改名為 `nginx`。
:::

## 日誌
Nginx 日誌對於監控、排查問題和維護您的 Web 伺服器至關重要。它們提供了有關伺服器效能、使用者行為和潛在問題的寶貴見解。

### AccessLogPath

- 類型：`string`

此選項用於為 Nginx UI 設定 Nginx 存取日誌的路徑，以便我們線上檢視日誌內容。

::: tip 提示
在 v2 版本中，我們會讀取 `nginx -V` 命令的輸出，以取得 Nginx 存取日誌的預設路徑。

如果您需要設定不同的路徑，您可以使用此選項。
:::

### ErrorLogPath

- 類型：`string`

此選項用於為 Nginx UI 設定 Nginx 錯誤日誌的路徑，以便我們線上檢視日誌內容。

::: tip 提示
在 v2 版本中，我們會讀取 `nginx -V` 命令的輸出，以取得 Nginx 錯誤日誌的預設路徑。

如果您需要設定不同的路徑，您可以使用此選項。
:::

### LogDirWhiteList

- 類型：`[]string`
- 版本：`>= v2.0.0-beta.36`
- 範例：`/var/log/nginx,/var/log/sites`

此選項用於為 Nginx UI 設定日誌檢視器的目錄白名單。

::: warning 警告
出於安全原因，您必須指定儲存日誌的目錄。

只有這些目錄中的日誌可以線上檢視。
:::

## 服務監控與控制

在本節中，我們將會介紹 Nginx UI 中關於 Nginx 服務的監控和控制命令的設定選項。

### ConfigDir
- 類型：`string`

此選項用於設定 Nginx 設定資料夾的路徑。

在 v2 版本中，我們會讀取 `nginx -V` 命令的輸出，以取得 Nginx 設定檔的預設路徑。

如果您需要覆蓋預設路徑，您可以使用此選項。

### PIDPath
- 類型：`string`

此選項用於設定 Nginx PID 文件的路徑。Nginx UI 將透過判斷該文件是否存在來判斷 Nginx 服務的執行狀態。

在 v2 版本中，我們會讀取 `nginx -V` 命令的輸出，以取得 Nginx PID 文件的預設路徑。

如果您需要覆蓋預設路徑，您可以使用此選項。

### TestConfigCmd
- 類型：`string`
- 預設值：`nginx -t`

此選項用於設定 Nginx 測試設定的命令。

### ReloadCmd
- 類型：`string`
- 預設值：`nginx -s reload`

此選項用於設定 Nginx 重新載入設定的命令。

### RestartCmd
- 類型：`string`

::: tip 提示
我們建議使用 systemd 管理 Nginx 的使用者，將這個值設定為 `systemctl restart nginx`。
否則，當您在 Nginx UI 中重啟 Nginx 後，將無法在 systemctl 中取得 Nginx 的準確狀態。
:::

若此選項為空，則 Nginx UI 將使用以下命令關閉 Nginx 服務：

```bash
start-stop-daemon --stop --quiet --oknodo --retry=TERM/30/KILL/5 --pidfile $PID
```

若無法從 `nginx -V` 中獲得 `--sbin-path` 路徑，則 Nginx UI 將使用以下命令開啟 Nginx 服務：

```bash
start-stop-daemon --start --quiet --pidfile $PID --exec $SBIN_PATH
```

## Stub Status

在本節中，我們將會介紹 Nginx UI 中關於 Nginx stub status 模組的設定選項。

### StubStatusPort
- 類型：`uint`
- 預設值：`51820`
- 版本：`>= v2.0.0-rc.6`

此選項用於設定 Nginx stub status 模組的連接埠。stub status 模組提供了 Nginx 的基本狀態資訊，Nginx UI 使用這些資訊來監控伺服器的效能。

::: tip 提示
請確保您設定的連接埠未被其他服務佔用。
:::
