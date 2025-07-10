# 即刻開始

## 嘗試一下

您可以透過 [演示](https://demo.nginxui.com) 直接試用 Nginx UI。

- 使用者名稱：admin
- 密碼：admin

## 使用前注意

Nginx UI 遵循 Debian 的網頁伺服器設定檔案標準。建立的網站設定檔案將會放置於 Nginx
設定資料夾（自動偵測）下的 `sites-available` 中，啟用後的網站將會建立一份設定檔案軟連結檔到 `sites-enabled`
資料夾。您可能需要提前調整設定檔案的組織方式。

對於非 Debian (及 Ubuntu) 作業系統，您可能需要將 `nginx.conf` 設定檔案中的內容修改為如下所示的 Debian 風格。

```nginx
http {
	# ...
	include /etc/nginx/conf.d/*.conf;
	include /etc/nginx/sites-enabled/*;
}
```

更多資訊請參閱：[debian/conf/nginx.conf](https://salsa.debian.org/nginx-team/nginx/-/blob/master/debian/conf/nginx.conf#L59-L60)

## 安裝

我們提供多種安裝方式以滿足不同需求：

- **macOS/Linux**: 使用 [Homebrew](./install-homebrew) 最簡單的安裝方式
- **Windows**: 使用 [Winget](./install-winget) Windows 套件管理員安裝
- **Linux**: 使用 [安裝指令碼](./install-script-linux) 直接控制主機上的 Nginx
- **Docker**: 透過 [Docker 安裝](#使用-docker) 使用我們提供的包含 Nginx 的映象
- **高階使用者**: 從 [最新發行版](https://github.com/0xJacky/nginx-ui/releases/latest) 下載並 [透過執行檔案執行](#透過執行檔案執行)，或者 [手動建構](./build)

第一次執行 Nginx UI 時，請在瀏覽器中存取 `http://<your_server_ip>:<listen_port>` 完成後續設定。

此外，我們提供了一個使用 Nginx 反向代理 Nginx UI 的 [範例](./nginx-proxy-example)，您可在安裝完成後使用。

## 使用 Homebrew 安裝

對於 macOS 和 Linux 使用者，您可以使用 Homebrew 安裝 Nginx UI，這是最簡單的安裝方式。

::: tip 提示

此安裝方式適用於 macOS 和 Linux。對於其他作業系統，請使用其他安裝方式。

:::

### 安裝

```bash
brew install 0xjacky/tools/nginx-ui
```

### 啟動服務

```bash
# 啟動服務
brew services start nginx-ui

# 或者在前台執行
nginx-ui
```

### 停止服務

```bash
brew services stop nginx-ui
```

### 升級

```bash
brew upgrade nginx-ui
```

### 解除安裝

```bash
# 首先停止服務
brew services stop nginx-ui

# 解除安裝軟體包
brew uninstall nginx-ui

# 可選：移除 tap
brew untap 0xjacky/tools
```

::: warning 警告

解除安裝後，設定檔案和資料將保留在：
- **macOS**: `~/Library/Application Support/nginx-ui/`
- **Linux**: `~/.local/share/nginx-ui/` 或 `~/.config/nginx-ui/`

如果您想要完全刪除所有資料，請手動刪除這些目錄。

:::

## 使用 Docker

您可以在 docker 中使用我們提供的 `uozi/nginx-ui:latest` [映像檔](https://hub.docker.com/r/uozi/nginx-ui)
，此映像檔基於 `nginx:latest` 建構。您可以直接將其監聽到 80 和 443 連接埠以取代宿主機上的 Nginx。

::: tip 提示

預設情況下，Nginx UI 會被反向代理到容器的 `8080` 連接埠。
首次使用時，對映到 `/etc/nginx` 的目錄必須為空資料夾。
如果你想要託管靜態檔案，可以直接將資料夾對映入容器中。

:::

::: warning 警告

如果您想要管理主機上的 Nginx，請選擇其他安裝方式。
如果您在使用 Linux，我們建議使用 [安裝指令碼](./install-script-linux) 安裝。

:::

### Docker 部署範例

```bash
docker run -dit \
  --name=nginx-ui \
  --restart=always \
  -e TZ=Asia/Shanghai \
  -v /mnt/user/appdata/nginx:/etc/nginx \
  -v /mnt/user/appdata/nginx-ui:/etc/nginx-ui \
  -v /var/www:/var/www \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -p 8080:80 -p 8443:443 \
  uozi/nginx-ui:latest
```

在這個範例中，容器的`80`連接埠和`443`連接埠分別對映到主機的`8080`連接埠和`8443`連接埠。
您需要存取`http://<your_server_ip>:8080`來存取 Nginx UI。

## 透過執行檔案執行

不建議直接執行 Nginx UI 可執行檔案用於非測試目的。
我們建議在 Linux 上將其設定為守護程式或使用 [安裝指令碼](./install-script-linux)。

### 設定

```shell
echo '[server]\nPort = 9000' > app.ini
```

::: tip 提示

在沒有 `app.ini` 時 Nginx UI 仍然可以啟動，它將使用預設偵聽連接埠 `9000`。

:::

### 執行

::: code-group

```shell [終端]
nginx-ui -config app.ini
```

```shell [背景]
nohup ./nginx-ui -config app.ini &
```

:::


### 停止

::: code-group

```shell [終端]
^C   # 按住 Ctrl+C
```

```shell [背景]
kill -9 $(ps -aux | grep nginx-ui | grep -v grep | awk '{print $2}')
```

:::
