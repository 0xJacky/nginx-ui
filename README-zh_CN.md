<div align="center">
      <img src="resources/logo.png" alt="Nginx UI Logo">
</div>

# Nginx UI

Yet another Nginx Web UI, developed by [0xJacky](https://jackyu.cn/) and [Hintay](https://blog.kugeek.com/).

[![Build and Publish](https://github.com/0xJacky/nginx-ui/actions/workflows/build.yml/badge.svg)](https://github.com/0xJacky/nginx-ui/actions/workflows/build.yml)

[For English](README.md)

## 关于项目
![](resources/screenshots/dashboard.png)
### 特色
- 在线查看服务器 CPU、内存、load average、磁盘使用率等指标
- 一键申请和自动续签 Let's encrypt 证书
- 在线编辑 Nginx 配置文件，编辑器支持 Nginx 配置语法高亮
- 使用 Go 和 Vue 开发，发行版本为单个可执行的二进制文件

### 国际化
- 英语
- 简体中文
- 繁体中文

我们欢迎您将项目翻译成任何语言。

### 构建基于
- [The Go Programming Language](https://go.dev/)
- [Gin Web Framework](https://gin-gonic.com)
- [GORM](http://gorm.io/index.html)
- [Vue 2](https://vuejs.org)
- [vue-gettext](https://github.com/Polyconseil/vue-gettext)

## 快速开始

### 使用前注意

Nginx UI 遵循 Nginx 的标准，创建的网站配置文件位于 Nginx 配置目录（自动检测）下的 `sites-available` 目录，
启用后的网站的配置文件将会创建一份软连接到 `sites-enabled`
目录中。因此，您可能需要提前调整配置文件的组织方式。

## 安装

### 平台支持
Nginx UI 可在以下平台中可用：
- Mac OS X 10.10 Yosemite 及之后版本（amd64 / arm64）;
- Linux 2.6.23 及之后版本（x86 / amd64 / arm64）；
  - 包括但不限于 Debian 7 / 8、Ubuntu 12.04 / 14.04 及后续版本、CentOS 6 / 7、Arch Linux；
- FreeBSD (x86 / amd64)；
- OpenBSD (x86 / amd64)；
- Dragonfly BSD (amd64)；

您可以在 [latest release](https://github.com/0xJacky/nginx-ui/releases/latest) 中下载最新发行版本

### 使用方法

#### 通过手动安装
- 在前台运行 Nginx UI
```shell
nginx-ui --config app.ini
```
- 关闭在前台运行的 Nginx UI
    - 键盘快捷键 `Ctrl+C`

- 后台运行 Nginx UI
```shell
nohup ./nginx-ui --config app.ini &
```
- 关闭在后台运行的 Nginx UI
```shell
jobs
kill %job_id
```
#### 通过脚本安装
- 启动 Nginx UI
```shell
systemctl start nginx-ui
```
- 停止 Nginx UI
```shell
systemctl stop nginx-ui
```
- 重启 Nginx UI
```shell
systemctl restart nginx-ui
```

## 手动构建
对于一键安装脚本不支持的平台，可以尝试手动构建。

### 依赖
- Make

- Golang 1.17+

- node.js 14+

  ```shell
  npx browserslist@latest --update-db
  ```
### 构建前端

请在 `frontend` 目录中执行以下命令。

```shell
yarn install
make translations
yarn build
```

### 构建后端

请先完成前端编译，再回到项目的根目录执行以下命令。

```shell
go build -o nginx-ui -v main.go
```

### Linux 一键安装脚本

**安装或升级**

```shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) @ install
```

一键安装脚本默认设置的监听端口为 `9000`，HTTP Challenge 端口默认为 `9180`，
如果出现端口冲突请进入 `/usr/local/etc/nginx-ui/app.ini` 修改，
并使用 `systemctl restart nginx-ui` 重启 Nginx UI 服务。

服务启动成功后，在浏览器中访问 `http://<your_server_ip>:9000/install` 完成后续配置。

**卸载 Nginx UI 但保留配置和数据库文件**

```shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) @ remove
```

### 更多用法

````shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) @ help
````

## Nginx 反向代理配置示例

```nginx
server {
    listen	80;
    listen	[::]:80;

    server_name	<your_server_name>;
    rewrite ^(.*)$  https://$host$1 permanent;
}

server {
    listen	443 ssl http2;
    listen	[::]:443 ssl http2;

    server_name	<your_server_name>;

    ssl_certificate	/path/to/ssl_cert;
    ssl_certificate_key	/path/to/ssl_cert_key;

    location / {
        proxy_set_header Host $host;
        proxy_set_header   X-Real-IP            $remote_addr;
        proxy_set_header   X-Forwarded-For      $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto    $scheme;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection upgrade;
        proxy_pass http://127.0.0.1:9000/;
    }
}
```

## 贡献

贡献使开源社区成为学习、启发和创造的绝佳场所。我们**非常感谢**您所做的任何贡献。

如果您有让这个项目变得更强的建议，欢迎 fork 这个仓库并创建一个 Pull Request。

您也可以创建一个带有 `enhancement` 的 Issue。最后，不要忘记给我们的项目<del>一键三连</del>点个 Star！再次感谢！

1. Fork 项目
2. 创建您的分支 (`git checkout -b feature/AmazingFeature`)
3. 提交您的修改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到您的分支 (`git push origin feature/AmazingFeature`)
5. 创建一个 Pull Request

## 开源许可

这个项目是在 GNU Affero Public License v3.0 (GPLv3) 许可下提供的，您可以在  [LICENSE](LICENSE) 找到。
通过使用、分发或对本项目做出贡献，表明您已同意本许可证的条款和条件。
