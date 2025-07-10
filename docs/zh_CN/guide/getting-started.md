# 即刻开始

## 尝试一下

您可以通过 [演示](https://demo.nginxui.com) 直接试用 Nginx UI。

- 用户名：admin
- 密码：admin

## 使用前注意

Nginx UI 遵循 Debian 的网页服务器配置文件标准。创建的网站配置文件将会放置于 Nginx
配置文件夹（自动检测）下的 `sites-available` 中，启用后的网站将会创建一份配置文件软连接到 `sites-enabled`
文件夹。您可能需要提前调整配置文件的组织方式。

对于非 Debian (及 Ubuntu) 系统，您可能需要将 `nginx.conf` 配置文件中的内容修改为如下所示的 Debian 风格。

```nginx
http {
	# ...
	include /etc/nginx/conf.d/*.conf;
	include /etc/nginx/sites-enabled/*;
}
```

更多信息请参阅：[debian/conf/nginx.conf](https://salsa.debian.org/nginx-team/nginx/-/blob/master/debian/conf/nginx.conf#L59-L60)

## 安装

我们提供多种安装方式以满足不同需求：

- **macOS/Linux**: 使用 [Homebrew](./install-homebrew) 最简单的安装方式
- **Windows**: 使用 [Winget](./install-winget) Windows 包管理器安装
- **Linux**: 使用 [安装脚本](./install-script-linux) 直接控制主机上的 Nginx
- **Docker**: 通过 [Docker 安装](#使用-docker) 使用我们提供的包含 Nginx 的镜像
- **高级用户**: 从 [最新发行版](https://github.com/0xJacky/nginx-ui/releases/latest) 下载并 [通过执行文件运行](#通过执行文件运行)，或者 [手动构建](./build)

第一次运行 Nginx UI 时，请在浏览器中访问 `http://<your_server_ip>:<listen_port>` 完成后续配置。

此外，我们提供了一个使用 Nginx 反向代理 Nginx UI 的 [示例](./nginx-proxy-example)，您可在安装完成后使用。

## 使用 Homebrew 安装

对于 macOS 和 Linux 用户，您可以使用 Homebrew 安装 Nginx UI，这是最简单的安装方式。

::: tip 提示

此安装方式适用于 macOS 和 Linux。对于其他操作系统，请使用其他安装方式。

:::

### 安装

```bash
brew install 0xjacky/tools/nginx-ui
```

### 启动服务

```bash
# 启动服务
brew services start nginx-ui

# 或者在前台运行
nginx-ui
```

### 停止服务

```bash
brew services stop nginx-ui
```

### 升级

```bash
brew upgrade nginx-ui
```

### 卸载

```bash
# 首先停止服务
brew services stop nginx-ui

# 卸载软件包
brew uninstall nginx-ui

# 可选：移除 tap
brew untap 0xjacky/tools
```

::: warning 警告

卸载后，配置文件和数据将保留在：
- **macOS**: `~/Library/Application Support/nginx-ui/`
- **Linux**: `~/.local/share/nginx-ui/` 或 `~/.config/nginx-ui/`

如果您想要完全删除所有数据，请手动删除这些目录。

:::

## 使用 Docker

您可以在 docker 中使用我们提供的 `uozi/nginx-ui:latest` [镜像](https://hub.docker.com/r/uozi/nginx-ui)
，此镜像基于 `nginx:latest` 构建。您可以直接将其监听到 80 和 443 端口以取代宿主机上的 Nginx。

::: tip 提示

默认情况下，Nginx UI 会被反向代理到容器的 `8080` 端口。
首次使用时，映射到 `/etc/nginx` 的目录必须为空文件夹。
如果你想要托管静态文件，可以直接将文件夹映射入容器中。

:::

::: warning 警告

如果您想要管理主机上的 Nginx，请选择其他安装方式。
如果您在使用 Linux，我们建议使用 [安装脚本](./install-script-linux) 安装。

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
  -v /var/run/docker.sock:/var/run/docker.sock \
  -p 8080:80 -p 8443:443 \
  uozi/nginx-ui:latest
```

在这个示例中，容器的`80`端口和`443`端口分别映射到主机的`8080`端口和`8443`端口。
您需要访问`http://<your_server_ip>:8080`来访问 Nginx UI。

## 通过执行文件运行

不建议直接运行 Nginx UI 可执行文件用于非测试目的。
我们建议在 Linux 上将其配置为守护进程或使用 [安装脚本](./install-script-linux)。

### 配置

```shell
echo '[server]\nPort = 9000' > app.ini
```

::: tip 提示

在没有 `app.ini` 时服务器仍然可以启动，它将默认侦听端口 `9000`。

:::

### 运行

::: code-group

```shell [终端]
nginx-ui -config app.ini
```

```shell [后台]
nohup ./nginx-ui -config app.ini &
```

:::


### 停止

::: code-group

```shell [终端]
^C   # 按住 Ctrl+C
```

```shell [后台]
kill -9 $(ps -aux | grep nginx-ui | grep -v grep | awk '{print $2}')
```

:::
