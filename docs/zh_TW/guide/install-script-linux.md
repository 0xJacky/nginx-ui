# 安裝指令碼

此 shell 指令碼僅適用於 Linux 作業系統。如果您使用的是其他作業系統，請參考 [快速入門](./getting-started) 指南進行手動安裝或使用 Docker。

## 安裝或升級

### `install.sh install`

安裝或更新 Nginx UI。

### 用法

```shell
install.sh install [OPTIONS]
```

### 選項

| 選項                    |                                                                                       |
|-----------------------|---------------------------------------------------------------------------------------|
| `-l, --local <file>`  | 從本機檔案安裝 Nginx UI (`string`)                                                           |
| `-p, --proxy <url>`   | 透過代理伺服器下載 (`string`)<br/>例如：`-p http://127.0.0.1:8118` 或 `-p socks5://127.0.0.1:1080` |
| `-r, --reverse-proxy` | 透過反向代理伺服器下載 (`string`)<br/>例如：`-r https://cloud.nginxui.com/`                               |
| `-c, --channel <channel>` | 指定版本通道 (`string`)<br/>可用通道：`stable`（預設）、`prerelease`、`dev`

#### 版本通道

| 通道         | 說明                                                      |
|------------|-----------------------------------------------------------|
| `stable`   | 最新穩定版本（預設） - 建議用於正式環境                                |
| `prerelease` | 最新預發布版本 - 包含正在測試的新功能，將在穩定版本發布前進行驗證                |
| `dev`      | 來自 dev 分支的最新開發構建 - 包含最新功能但可能不穩定                   |

### 快速使用

::: code-group

```shell [穩定版（預設）]
# 安裝最新穩定版本
bash -c "$(curl -L https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ install
```

```shell [預發布版]
# 安裝最新預發布版本
bash -c "$(curl -L https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ install --channel prerelease
```

```shell [開發版]
# 安裝最新開發構建
bash -c "$(curl -L https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ install --channel dev
```

:::

安裝指令碼預設的監聽連接埠為 `9000`，HTTP Challenge 連接埠預設為 `9180`。如果出現連接埠衝突請修改 `/usr/local/etc/nginx-ui/app.ini`，
並使用 `systemctl restart nginx-ui` 重啟 Nginx UI 守護行程。更多有關資訊，請檢視 [設定參考](./config-server)。

## 解除安裝

### `install.sh remove`

解除安裝 Nginx UI。

### 用法

```shell
install.sh remove [OPTIONS]
```

### 選項

| 選項        |                                       |
|-----------|---------------------------------------|
| `--purge` | 刪除所有 Nginx UI 檔案，包括日誌、設定等 (`boolean`) |

### 快速使用

::: code-group

```shell [移除]
# 解除安裝 Nginx UI 但保留設定和資料庫檔案
bash -c "$(curl -L https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ remove
```

```shell [清除]
# 解除安裝並刪除所有 Nginx UI 檔案，包括設定和資料庫檔案
bash -c "$(curl -L https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ remove --purge
```

:::

## 幫助

### `install.sh help`

顯示可用選項。

### 用法

```shell
install.sh help
```

### 快速使用

```shell
bash -c "$(curl -L https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ help
```

## 控制服務

透過此指令碼，Nginx UI 將作為服務安裝。安裝指令碼會檢測您系統的服務管理器並設置相應的服務控制機制。

### Systemd

如果您的系統使用 systemd，請使用以下 `systemctl` 指令控制：

::: code-group

```shell [啟動]
systemctl start nginx-ui
```

```shell [停止]
systemctl stop nginx-ui
```

```shell [重啟]
systemctl restart nginx-ui
```

```shell [顯示狀態]
systemctl status nginx-ui
```

```shell [開機啟動]
systemctl enable nginx-ui
```

:::

### OpenRC

如果您的系統使用 OpenRC，請使用以下 `rc-service` 指令控制：

::: code-group

```shell [啟動]
rc-service nginx-ui start
```

```shell [停止]
rc-service nginx-ui stop
```

```shell [重啟]
rc-service nginx-ui restart
```

```shell [顯示狀態]
rc-service nginx-ui status
```

```shell [開機啟動]
rc-update add nginx-ui default
```

:::

### Init.d

如果您的系統使用傳統的 init.d 指令碼，請使用以下指令控制：

::: code-group

```shell [啟動]
/etc/init.d/nginx-ui start
```

```shell [停止]
/etc/init.d/nginx-ui stop
```

```shell [重啟]
/etc/init.d/nginx-ui restart
```

```shell [顯示狀態]
/etc/init.d/nginx-ui status
```

:::