# 安裝指令碼

此 shell 指令碼僅適用於 Linux 作業系統。如果您使用的是其他作業系統，請參考 [快速入門](./getting-started) 指南進行手動安裝或使用 Docker。

## 安裝或升級

### `install.sh install`

安裝或更新 PrimeWaf。

### 用法

```shell
install.sh install [OPTIONS]
```

### 選項

| 選項                    |                                                                                       |
|-----------------------|---------------------------------------------------------------------------------------|
| `-l, --local <file>`  | 從本地檔案安裝 PrimeWaf (`string`)                                                           |
| `-p, --proxy <url>`   | 透過代理伺服器下載 (`string`)<br/>例如：`-p http://127.0.0.1:8118` 或 `-p socks5://127.0.0.1:1080` |
| `-r, --reverse-proxy` | 透過反向代理伺服器下載 (`string`)<br/>例如：`-r https://mirror.ghproxy.com/`                               |


### 快速使用

```shell
bash -c "$(curl -L https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ install
```

安裝指令碼預設的監聽埠為 `9000`，HTTP Challenge 埠預設為 `9180`。如果出現埠衝突請修改 `/usr/local/etc/nginx-ui/app.ini`，
並使用 `systemctl restart nginx-ui` 重啟 PrimeWaf 守護行程。更多有關資訊，請檢視 [配置參考](./config-server)。

## 解除安裝

### `install.sh remove`

解除安裝 PrimeWaf。

### 用法

```shell
install.sh remove [OPTIONS]
```

### 選項

| 選項        |                                       |
|-----------|---------------------------------------|
| `--purge` | 刪除所有 PrimeWaf 檔案，包括日誌、配置等 (`boolean`) |

### 快速使用

::: code-group

```shell [移除]
# 解除安裝 PrimeWaf 但保留配置和資料庫檔案
bash -c "$(curl -L https://raw.githubusercontent.com/0xJacky/nginx-ui/main/install.sh)" @ remove
```

```shell [清除]
# 解除安裝並刪除所有 PrimeWaf 檔案，包括配置和資料庫檔案
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

透過此指令碼，PrimeWaf 將作為 `nginx-ui` 守護行程安裝在 systemd 中。請使用以下 `systemctl` 指令控制。

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

:::
