<div align="center">
      <img src="resources/logo.png" alt="Nginx UI Logo">
</div>

# Nginx UI

Yet another Nginx Web UI

Nginx 网络管理界面，由  [0xJacky](https://jackyu.cn/) 与 [Hintay](https://blog.kugeek.com/) 开发。

[![Build and Publish](https://github.com/0xJacky/nginx-ui/actions/workflows/build.yml/badge.svg)](https://github.com/0xJacky/nginx-ui/actions/workflows/build.yml)

[English](README.md) | [Español](README-es.md) | 简体中文 | [繁體中文](README-zh_TW.md)

<details>
  <summary>目录</summary>
  <ol>
    <li>
      <a href="#关于项目">关于项目</a>
      <ul>
        <li><a href="#在线预览">在线预览</a></li>
        <li><a href="#特色">特色</a></li>
        <li><a href="#国际化">国际化</a></li>
        <li><a href="#构建基于">构建基于</a></li>
      </ul>
    </li>
    <li>
      <a href="#入门指南">入门指南</a>
      <ul>
        <li><a href="#使用前注意">使用前注意</a></li>
        <li><a href="#安装">安装</a></li>
        <li>
          <a href="#使用方法">使用方法</a>
          <ul>
            <li><a href="#通过执行文件运行">通过执行文件运行</a></li>
            <li><a href="#使用-systemd">使用 Systemd</a></li>
            <li><a href="#使用-Docker">使用 Docker</a></li>
          </ul>
        </li>
      </ul>
    </li>
    <li>
      <a href="#手动构建">手动构建</a>
      <ul>
        <li><a href="#依赖">依赖</a></li>
        <li><a href="#构建前端">构建前端</a></li>
        <li><a href="#构建后端">构建后端</a></li>
      </ul>
    </li>
    <li>
      <a href="#linux-安装脚本">Linux 安装脚本</a>
      <ul>
        <li><a href="#基本用法">基本用法</a></li>
        <li><a href="#更多用法">更多用法</a></li>
      </ul>
    </li>
    <li><a href="#nginx-反向代理配置示例">Nginx 反向代理配置示例</a></li>
    <li><a href="#贡献">贡献</a></li>
    <li><a href="#开源许可">开源许可</a></li>
  </ol>
</details>


## 关于项目

![Dashboard](resources/screenshots/dashboard_zh_CN.png)

### 在线预览
网址：[https://demo.nginxui.com](https://demo.nginxui.com)
- 用户名：admin
- 密码：admin

### 特色

- 在线查看服务器 CPU、内存、系统负载、磁盘使用率等指标
- 在线 ChatGPT 助理
- 一键申请和自动续签 Let's encrypt 证书
- 在线编辑 Nginx 配置文件，编辑器支持 Nginx 配置语法高亮
- 在线查看 Nginx 日志
- 使用 Go 和 Vue 开发，发行版本为单个可执行的二进制文件
- 保存配置后自动测试配置文件并重载 Nginx
- 基于网页浏览器的高级命令行终端
- 支持深色模式
- 自适应网页设计

### 国际化

- 英语
- 简体中文
- 繁体中文

我们欢迎您将项目翻译成任何语言。

### 构建基于
- [The Go Programming Language](https://go.dev)
- [Gin Web Framework](https://gin-gonic.com)
- [GORM](http://gorm.io)
- [Vue 3](https://v3.vuejs.org)
- [Vite](https://vitejs.dev)
- [TypeScript](https://www.typescriptlang.org/)
- [Ant Design Vue](https://antdv.com)
- [vue3-gettext](https://github.com/jshmrtn/vue3-gettext)
- [vue3-ace-editor](https://github.com/CarterLi/vue3-ace-editor)
- [Gonginx](https://github.com/tufanbarisyildirim/gonginx)

## 入门指南

### 使用前注意

Nginx UI 遵循 Debian 的网页服务器配置文件标准。创建的网站配置文件将会放置于 Nginx 配置文件夹（自动检测）下的 `sites-available` 中，启用后的网站将会创建一份配置文件软连接到 `sites-enabled` 文件夹。您可能需要提前调整配置文件的组织方式。

对于非 Debian (及 Ubuntu) 系统，您可能需要将 `nginx.conf` 配置文件中的内容修改为如下所示的 Debian 风格。

```nginx
http {
	# ...
	include /etc/nginx/conf.d/*.conf;
	include /etc/nginx/sites-enabled/*;
}
```

更多信息请参阅：[debian/conf/nginx.conf](https://salsa.debian.org/nginx-team/nginx/-/blob/master/debian/conf/nginx.conf#L59-L60)

### 安装

Nginx UI 可在以下平台中使用：

- Mac OS X 10.10 Yosemite 及之后版本（amd64 / arm64）
- Linux 2.6.23 及之后版本（x86 / amd64 / arm64 / armv5 / armv6 / armv7）
  - 包括但不限于 Debian 7 / 8、Ubuntu 12.04 / 14.04 及后续版本、CentOS 6 / 7、Arch Linux
- FreeBSD
- OpenBSD
- Dragonfly BSD
- Openwrt

您可以在 [最新发行 (latest release)](https://github.com/0xJacky/nginx-ui/releases/latest) 中下载最新版本，或使用 [Linux 安装脚本](#linux-安装脚本)。

### 使用方法

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
如果你使用的是[Linux 安装脚本](#linux-安装脚本)，Nginx UI 将作为 `nginx-ui` 服务安装在 systemd 中。请使用 `systemctl` 命令控制。

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

您可以在 docker 中使用我们提供的 `uozi/nginx-ui:latest` [镜像](https://hub.docker.com/r/uozi/nginx-ui)，此镜像基于 `nginx:latest` 构建。您可以直接将其监听到 80 和 443 端口以取代宿主机上的 Nginx。

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

### 依赖

- Make

- Golang 1.22+

- node.js 21+

  ```shell
  npx browserslist@latest --update-db
  ```

### 构建前端

请在 `app` 目录中执行以下命令。

```shell
pnpm install
pnpm build
```

### 构建后端

请先完成前端编译，再回到项目的根目录执行以下命令。

```shell
go build -tags=jsoniter -ldflags "$LD_FLAGS -X 'github.com/0xJacky/Nginx-UI/settings.buildTime=$(date +%s)'" -o nginx-ui -v main.go
```

## Linux 安装脚本

### 基本用法

**安装或升级**

```shell
bash <(curl -L -s https://mirror.ghproxy.com/https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) install -r https://mirror.ghproxy.com/
```
一键安装脚本默认设置的监听端口为 `9000`，HTTP Challenge 端口默认为 `9180`，如果出现端口冲突请进入 `/usr/local/etc/nginx-ui/app.ini` 修改，并使用 `systemctl restart nginx-ui` 重启 Nginx UI 服务。

**卸载 Nginx UI 但保留配置和数据库文件**

```shell
bash <(curl -L -s https://mirror.ghproxy.com/https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) remove
```

### 更多用法

````shell
bash <(curl -L -s https://mirror.ghproxy.com/https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) help
````

## Nginx 反向代理配置示例

```nginx
server {
    listen          80;
    listen          [::]:80;

    server_name     <your_server_name>;
    rewrite ^(.*)$  https://$host$1 permanent;
}

map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen  443       ssl;
    listen  [::]:443  ssl;
    http2   on;

    server_name         <your_server_name>;

    ssl_certificate     /path/to/ssl_cert;
    ssl_certificate_key /path/to/ssl_cert_key;

    location / {
        proxy_set_header    Host                $host;
        proxy_set_header    X-Real-IP           $remote_addr;
        proxy_set_header    X-Forwarded-For     $proxy_add_x_forwarded_for;
        proxy_set_header    X-Forwarded-Proto   $scheme;
        proxy_http_version  1.1;
        proxy_set_header    Upgrade             $http_upgrade;
        proxy_set_header    Connection          $connection_upgrade;
        proxy_pass          http://127.0.0.1:9000/;
    }
}
```

## 贡献

贡献使开源社区成为学习、启发和创造的绝佳场所。我们**非常感谢**您所做的任何贡献。

如果您有让这个项目变得更强的建议，欢迎 fork 这个仓库并创建一个 Pull Request。您也可以创建一个带有 `enhancement` （优化）标签的 Issue。最后，不要忘记给我们的项目<del>一键三连</del>点个 Star！再次感谢！

1. Fork 项目
2. 创建您的分支 (`git checkout -b feature/AmazingFeature`)
3. 提交您的修改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到您的分支 (`git push origin feature/AmazingFeature`)
5. 创建一个 Pull Request

## 开源许可

此项目基于 GNU Affero Public License v3.0 (AGPLv3) 许可，请参阅 [LICENSE](LICENSE) 文件。通过使用、分发或对本项目做出贡献，表明您已同意本许可证的条款和条件。
