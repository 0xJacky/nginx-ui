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

### SbinPath
- 類型：`string`
- 版本：`>= v2.1.10`

此選項用於設定 Nginx 可執行檔的路徑。

預設情況下，Nginx UI 會嘗試在 `$PATH` 中查找 Nginx 可執行檔。

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

### StubStatusPort
- 類型：`uint`
- 預設值：`51820`
- 版本：`>= v2.0.0-rc.6`

此選項用於設定 Nginx stub status 模組的連接埠。stub status 模組提供了 Nginx 的基本狀態資訊，Nginx UI 使用這些資訊來監控伺服器的效能。

::: tip 提示
請確保您設定的連接埠未被其他服務佔用。
:::

## 維護頁面

### MaintenanceTemplate
- 類型：`string`
- 環境變數：`NGINX_UI_NGINX_MAINTENANCE_TEMPLATE`
- 範例：`maintenance.html`

此選項用於為 Nginx UI 維護頁面選擇自訂 HTML 模板。您可以透過環境變數設定，也可以在 Settings > Nginx 中設定。

此設定只使用檔案名稱。Nginx UI 會從 `/etc/nginx/maintenance/<filename>` 載入自訂模板，設定值中的路徑部分會被忽略。

如果此選項為空、檔案不可讀或檔案內容為空，Nginx UI 將回退到內建維護頁面模板。

對於 Docker 部署，請將主機目錄掛載到 `/etc/nginx/maintenance`，並將模板檔案放在該目錄中：

```yaml
services:
  nginx-ui:
    image: uozi/nginx-ui:latest
    volumes:
      - ./maintenance:/etc/nginx/maintenance
    environment:
      - NGINX_UI_NGINX_MAINTENANCE_TEMPLATE=maintenance.html
```

## 容器控制

在本節中，我們將會介紹 Nginx UI 中關於控制運行在另一個 Docker 容器中的 Nginx 服務的設定選項。

### ContainerName
- 類型：`string`
- 版本：`>= v2.0.0-rc.6`

此選項用於指定執行 Nginx 的 Docker 容器名稱。

如果此選項為空，Nginx UI 將控制本機或當前容器內的 Nginx 服務。

如果此選項不為空，Nginx UI 將控制執行在指定容器中的 Nginx 服務。

::: tip 提示
如果使用 Nginx UI 官方容器，想要控制另外一個容器裡的 Nginx，務必將宿主機內的 docker.sock 映射到 Nginx UI 官方容器中。

例如：`-v /var/run/docker.sock:/var/run/docker.sock`
:::

## 透過 SSH 控制宿主機 Nginx

對於 Nginx UI 執行在 Docker 容器中、而 Nginx 以原生方式安裝在宿主機上的部署場景（例如透過 apt/yum 安裝並由 systemd 管理），Nginx UI 提供了第三種控制模式，透過 SSH 執行命令並使用綁定掛載進行檔案 I/O。

### 限制

- **僅限同一宿主機**：Nginx UI 容器與目標 nginx 程序必須在同一台實體機或虛擬機上。如需多主機管理，請參閱 [叢集節點跨主機指南](cluster-node-cross-host.md)。
- 宿主機上**需要 systemd**。此模式透過呼叫 `systemctl reload|restart <unit>` 來控制服務。
- 宿主機的 nginx 使用者必須允許一個專用的非特權使用者（通常為 `nginxui`）透過 `sudo -n` 無密碼執行一組受限命令。

### 快速開始

1. 在 Web 介面中，前往**偏好設定 → Nginx**，選擇**透過 SSH 控制宿主機**模式，並開啟設定精靈。
2. 按照四步設定精靈操作：產生金鑰對、將產生的 docker-compose 片段貼到您的 stack 中、在宿主機上套用 sudoers/authorized_keys 片段，然後執行驗證。
3. 所有檢查通過後，儲存設定。

也可以使用命令列：

```bash
nginx-ui host-setup print --host-address host.docker.internal:22 --host-user nginxui
nginx-ui host-setup test
```

### 設定欄位

| 欄位 | 描述 |
|---|---|
| `host_mode` | 設定為 `ssh` 以啟用此模式 |
| `host_address` | 遠端 `host:port` |
| `host_user` | 宿主機上的 SSH 使用者 |
| `host_private_key_path` | 容器內的私鑰路徑 |
| `host_systemd_unit_name` | 預設為 `nginx.service` |
| `host_systemctl_path` | 預設為 `/bin/systemctl` |

另請參閱：[Host SSH 設定指引](host-via-ssh-setup.md)。
