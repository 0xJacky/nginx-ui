# 即刻開始

## 嘗試一下

您可以透過 [演示](https://demo.nginxui.com) 直接試用 Nginx UI。

- 使用者名稱：admin
- 密碼：admin

## 使用前注意

Nginx UI 遵循 Debian 的網頁伺服器配置檔案標準。建立的網站配置檔案將會放置於 Nginx
配置資料夾（自動檢測）下的 `sites-available` 中，啟用後的網站將會建立一份配置檔案軟連結檔到 `sites-enabled`
資料夾。您可能需要提前調整配置檔案的組織方式。

對於非 Debian (及 Ubuntu) 作業系統，您可能需要將 `nginx.conf` 配置檔案中的內容修改為如下所示的 Debian 風格。

```nginx
http {
	# ...
	include /etc/nginx/conf.d/*.conf;
	include /etc/nginx/sites-enabled/*;
}
```

更多資訊請參閱：[debian/conf/nginx.conf](https://salsa.debian.org/nginx-team/nginx/-/blob/master/debian/conf/nginx.conf#L59-L60)

## 安裝

我們建議Linux使用者使用 [安裝指令碼](./install-script-linux)，這樣您可以直接控制主機上的 Nginx。您也可以透過 [Docker 安裝](#使用-docker)，
我們提供的映象包含 Nginx 並可以直接使用。對於高階使用者，您也可以在 [最新發行 (latest release)](https://github.com/0xJacky/nginx-ui/releases/latest)
中下載最新版本並 [透過執行檔案執行](#透過執行檔案執行)，或者 [手動構建](./build)。

第一次執行 Nginx UI 時，請在瀏覽器中訪問 `http://<your_server_ip>:<listen_port>/install` 完成後續配置。

此外，我們提供了一個使用 Nginx 反向代理 Nginx UI 的 [示例](./nginx-proxy-example)，您可在安裝完成後使用。


## 使用 Docker

您可以在 docker 中使用我們提供的 `uozi/nginx-ui:latest` [映像檔](https://hub.docker.com/r/uozi/nginx-ui)
，此映像檔基於 `nginx:latest` 構建。您可以直接將其監聽到 80 和 443 埠以取代宿主機上的 Nginx。

::: tip 提示

預設情況下，Nginx UI 會被反向代理到容器的 `8080` 埠。
首次使用時，對映到 `/etc/nginx` 的目錄必須為空資料夾。
如果你想要託管靜態檔案，可以直接將資料夾對映入容器中。

:::

::: warning 警告

如果您想要管理主機上的 Nginx，請選擇其他安裝方式。
如果您在使用 Linux，我們建議使用 [安裝指令碼](./install-script-linux) 安裝。

:::

### Docker 部署示例

```bash
docker run -dit \
  --name=nginx-ui \
  --restart=always \
  -e TZ=Asia/Shanghai \
  -v /mnt/user/appdata/nginx:/etc/nginx \
  -v /mnt/user/appdata/nginx-ui:/etc/nginx-ui \
  -v /var/www:/var/www \
  -p 8080:80 -p 8443:443 \
  uozi/nginx-ui:latest
```

在這個示例中，容器的`8080`埠和`8443`埠分別映射到主機的`80`埠和`443`埠。
您需要訪問`http://<your_server_ip>:8080`來訪問 Nginx UI。

## 透過執行檔案執行

不建議直接執行 Nginx UI 可執行檔案用於非測試目的。
我們建議在 Linux 上將其配置為守護程序或使用 [安裝指令碼](./install-script-linux)。

### 配置

```shell
echo '[server]\nHttpPort = 9000' > app.ini
```

::: tip 提示

在沒有 `app.ini` 時 Nginx UI 仍然可以啟動，它將使用預設偵聽埠 `9000`。

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
