# 即刻开始

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

Nginx UI 可在以下平台中使用：

- Mac OS X 10.10 Yosemite 及之后版本（amd64 / arm64）
- Linux 2.6.23 及之后版本（x86 / amd64 / arm64 / armv5 / armv6 / armv7）
    - 包括但不限于 Debian 7 / 8、Ubuntu 12.04 / 14.04 及后续版本、CentOS 6 / 7、Arch Linux
- FreeBSD
- OpenBSD
- Dragonfly BSD
- Openwrt

您可以在 [最新发行 (latest release)](https://github.com/0xJacky/nginx-ui/releases/latest)
中下载最新版本，或使用 [Linux 安装脚本](#linux-安装脚本)。

## 使用方法

第一次运行 Nginx UI 时，请在浏览器中访问 `http://<your_server_ip>:<listen_port>/install` 完成后续配置。

#### 通过执行文件运行

**在终端中运行 Nginx UI**

```shell
nginx-ui -config app.ini
```

在终端使用 `Control+C` 退出 Nginx UI。

**在后台运行 Nginx UI**

```shell
nohup ./nginx-ui -config app.ini &
```

使用以下命令停止 Nginx UI。

```shell
kill -9 $(ps -aux | grep nginx-ui | grep -v grep | awk '{print $2}')
```

#### 使用 Systemd

如果你使用的是[Linux 安装脚本](#linux-安装脚本)，Nginx UI 将作为 `nginx-ui` 服务安装在 systemd 中。请使用 `systemctl`
命令控制。

**启动 Nginx UI**

```shell
systemctl start nginx-ui
```

**停止 Nginx UI**

```shell
systemctl stop nginx-ui
```

**重启 Nginx UI**

```shell
systemctl restart nginx-ui
```

#### 使用 Docker

您可以在 docker 中使用我们提供的 `uozi/nginx-ui:latest` [镜像](https://hub.docker.com/r/uozi/nginx-ui)
，此镜像基于 `nginx:latest` 构建。您可以直接将其监听到 80 和 443 端口以取代宿主机上的 Nginx。

注意：映射到 `/etc/nginx` 的文件夹应该为一个空目录。

#### 注意

1. 首次使用时，映射到 `/etc/nginx` 的目录必须为空文件夹。
2. 如果你想要托管静态文件，可以直接将文件夹映射入容器中。

**Docker 部署示例**

```bash
docker run -dit \
  --name=nginx-ui \
  --restart=always \
  -e TZ=Asia/Shanghai \
  -v /mnt/user/appdata/nginx:/etc/nginx \
  -v /mnt/user/appdata/nginx-ui:/etc/nginx-ui \
  -p 8080:80 -p 8443:443 \
  uozi/nginx-ui:latest
```

## 手动构建

对于没有官方构建版本的平台，可以尝试手动构建。

## 依赖

- Make

- Golang 1.19+

- node.js 18+

  ```shell
  npx browserslist@latest --update-db
  ```

## 构建前端

请在 `frontend` 目录中执行以下命令。

```shell
yarn install
make translations
yarn build
```

## 构建后端

请先完成前端编译，再回到项目的根目录执行以下命令。

```shell
go build -o nginx-ui -v main.go
```

## Linux 安装脚本

## 基本用法

**安装或升级**

```shell
bash <(curl -L -s https://ghproxy.com/https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) install -r https://ghproxy.com/
```

一键安装脚本默认设置的监听端口为 `9000`，HTTP Challenge 端口默认为 `9180`
，如果出现端口冲突请进入 `/usr/local/etc/nginx-ui/app.ini` 修改，并使用 `systemctl restart nginx-ui` 重启 Nginx UI 服务。

**卸载 Nginx UI 但保留配置和数据库文件**

```shell
bash <(curl -L -s https://ghproxy.com/https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) remove
```

## 更多用法

````shell
bash <(curl -L -s https://ghproxy.com/https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) help
````
